package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 本文件覆盖：worker 池拒绝计费任务时（队列满、采样未选或已停止），
// handler 必须内联同步执行一次；worker 池自身的溢出统计策略保持不变。

type usageRecordTaskSubmitter struct {
	name   string
	submit func(pool *service.UsageRecordWorkerPool, parent context.Context, task service.UsageRecordTask)
}

func usageRecordTaskSubmitters() []usageRecordTaskSubmitter {
	return []usageRecordTaskSubmitter{
		{
			name: "gateway",
			submit: func(pool *service.UsageRecordWorkerPool, parent context.Context, task service.UsageRecordTask) {
				(&GatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask(parent, task)
			},
		},
		{
			name: "openai",
			submit: func(pool *service.UsageRecordWorkerPool, parent context.Context, task service.UsageRecordTask) {
				(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask(parent, task)
			},
		},
	}
}

func newStoppedUsageRecordPoolForTest() *service.UsageRecordWorkerPool {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:    1,
		QueueSize:      1,
		TaskTimeout:    time.Second,
		OverflowPolicy: config.UsageRecordOverflowPolicySync,
	})
	pool.Stop()
	return pool
}

func TestSubmitUsageRecordTask_StoppedPoolFallsBackToSync(t *testing.T) {
	for _, tt := range usageRecordTaskSubmitters() {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			tt.submit(newStoppedUsageRecordPoolForTest(), context.Background(), func(ctx context.Context) {
				calls.Add(1)
			})
			require.Equal(t, int32(1), calls.Load(), "池已停止时计费任务必须内联同步执行一次")
		})
	}
}

func TestSubmitUsageRecordTask_FullPoolDroppedFallsBackToSync(t *testing.T) {
	tests := []struct {
		name           string
		overflowPolicy string
		samplePercent  int
		primeSample    bool
	}{
		{name: "drop", overflowPolicy: config.UsageRecordOverflowPolicyDrop},
		{name: "sample_unselected", overflowPolicy: config.UsageRecordOverflowPolicySample, samplePercent: 1, primeSample: true},
	}

	for _, policy := range tests {
		for _, tt := range usageRecordTaskSubmitters() {
			t.Run(policy.name+"/"+tt.name, func(t *testing.T) {
				pool := newFullUsageRecordPoolForTest(t, policy.overflowPolicy, policy.samplePercent)
				if policy.primeSample {
					require.Equal(t, service.UsageRecordSubmitModeSync, pool.Submit(func(context.Context) {}))
				}
				var calls atomic.Int32
				tt.submit(pool, context.Background(), func(ctx context.Context) {
					calls.Add(1)
				})
				require.Equal(t, int32(1), calls.Load(), "队列满的计费任务必须内联同步执行一次")
				require.Equal(t, uint64(1), pool.Stats().DroppedQueueFull)
			})
		}
	}
}

func TestSubmitUsageRecordTask_FullPoolSampleSyncExecutesOnlyOnce(t *testing.T) {
	for _, tt := range usageRecordTaskSubmitters() {
		t.Run(tt.name, func(t *testing.T) {
			pool := newFullUsageRecordPoolForTest(t, config.UsageRecordOverflowPolicySample, 100)
			var calls atomic.Int32
			tt.submit(pool, context.Background(), func(ctx context.Context) {
				calls.Add(1)
			})
			require.Equal(t, int32(1), calls.Load(), "pool sync_fallback 已执行任务，handler 不得重复执行")
			require.Equal(t, uint64(1), pool.Stats().SyncFallbackTasks)
		})
	}
}

func TestSubmitUsageRecordTask_EnqueuedExecutesOnce(t *testing.T) {
	for _, tt := range usageRecordTaskSubmitters() {
		t.Run(tt.name, func(t *testing.T) {
			pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
				WorkerCount:    1,
				QueueSize:      1,
				TaskTimeout:    time.Second,
				OverflowPolicy: config.UsageRecordOverflowPolicyDrop,
			})
			t.Cleanup(pool.Stop)
			var calls atomic.Int32
			done := make(chan struct{})
			tt.submit(pool, context.Background(), func(ctx context.Context) {
				calls.Add(1)
				close(done)
			})
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("enqueued task did not execute")
			}
			require.Equal(t, int32(1), calls.Load())
		})
	}
}

func TestSubmitUsageRecordTask_DroppedFallbackCopiesContextAfterParentCancellation(t *testing.T) {
	for _, tt := range usageRecordTaskSubmitters() {
		t.Run(tt.name, func(t *testing.T) {
			pool := newFullUsageRecordPoolForTest(t, config.UsageRecordOverflowPolicyDrop, 0)
			parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "usage-record-client-request")
			parent = context.WithValue(parent, ctxkey.RequestID, "usage-record-request")
			parent, cancel := context.WithCancel(parent)
			cancel()

			var calls atomic.Int32
			tt.submit(pool, parent, func(ctx context.Context) {
				calls.Add(1)
				require.Equal(t, "usage-record-client-request", ctx.Value(ctxkey.ClientRequestID))
				require.Equal(t, "usage-record-request", ctx.Value(ctxkey.RequestID))
				require.NoError(t, ctx.Err(), "同步兜底不能继承已取消的请求 context")
				_, ok := ctx.Deadline()
				require.True(t, ok, "同步兜底必须保留 deadline")
			})
			require.Equal(t, int32(1), calls.Load())
		})
	}
}

func newFullUsageRecordPoolForTest(t *testing.T, overflowPolicy string, samplePercent int) *service.UsageRecordWorkerPool {
	t.Helper()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Minute,
		OverflowPolicy:        overflowPolicy,
		OverflowSamplePercent: samplePercent,
		AutoScaleEnabled:      false,
	})
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		pool.Stop()
	})
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(ctx context.Context) {
		close(started)
		<-release
	}))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(ctx context.Context) {
		<-release
	}))
	return pool
}
