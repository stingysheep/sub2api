package main

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGeneratedWireIncludesShepRuntimeWiring(t *testing.T) {
	generated, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)
	require.Contains(t, string(generated), ":= provideShepRuntimeWiring(gatewayService, openAIGatewayService, affiliateService, usageHandler, dashboardService)")
}

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cleanup := minimalDependencyCleanup(nil, &service.UsageRecordWorkerPool{})
	require.NotPanics(t, cleanup)
}

type cleanupBillingRepository struct {
	service.UsageBillingRepository
	closeFn func()
}

func (r *cleanupBillingRepository) Close() { r.closeFn() }

func TestProvideCleanup_DrainsUsageWorkerBeforeClosingJournal(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second,
	})
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseTask := func() { releaseOnce.Do(func() { close(release) }) }
	defer func() { releaseTask(); pool.Stop() }()
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {
		close(started)
		<-release
	}))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("usage task did not start")
	}
	type closeState struct {
		completed uint64
		mode      service.UsageRecordSubmitMode
	}
	closed := make(chan closeState, 1)
	repo := &cleanupBillingRepository{closeFn: func() {
		closed <- closeState{pool.Stats().CompletedTasks, pool.Submit(func(context.Context) {})}
	}}
	cleanup := minimalDependencyCleanup(repo, pool)
	done := make(chan struct{})
	go func() { cleanup(); close(done) }()
	select {
	case <-closed:
		t.Fatal("journal closed while usage task was still running")
	case <-time.After(100 * time.Millisecond):
	}
	releaseTask()
	select {
	case state := <-closed:
		require.Equal(t, uint64(1), state.completed)
		require.Equal(t, service.UsageRecordSubmitModeDroppedStopped, state.mode)
	case <-time.After(5 * time.Second):
		t.Fatal("journal did not close after usage task drained")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish")
	}
}

func minimalDependencyCleanup(usageBillingRepo service.UsageBillingRepository, usagePool *service.UsageRecordWorkerPool) func() {
	cfg := &config.Config{}

	oauthSvc := service.NewOAuthService(nil, nil)
	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	codexVersionSyncSvc := service.NewOpenAICodexVersionSyncService(nil, nil, nil, time.Second)
	proxyExpirySvc := service.NewProxyExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)

	return provideCleanup(
		nil, // entClient
		usageBillingRepo,
		shepRuntimeWiring{},
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		nil, // opsService
		nil, // opsIngressRejectAggregator
		nil, // apiKeyService
		nil, // authCacheInvalidationWorker
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		nil, // cnProviderBalanceCheck
		codexVersionSyncSvc,
		proxyExpirySvc,
		subscriptionExpirySvc,
		&service.UsageCleanupService{},
		idempotencyCleanupSvc,
		&service.BatchImageCleanupService{},
		nil, // batchImageWorker
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		usagePool,
		&service.SubscriptionService{},
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil, // grokOAuth
		nil, // openAIGateway
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // channelMonitorRunner
		nil, // channelMonitorV2Aggregator
		nil, // quotaFlusher
		nil, // upstreamBillingProbe
		nil, // ollamaCloudUsage
		nil, // auditLog
		nil, // openAIAutoReset
		nil, // promptAudit
		nil, // pluginManager
	)
}
