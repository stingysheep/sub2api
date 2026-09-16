package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompatStreamGuardFirstOutputInterruptsBody(t *testing.T) {
	ctx, guard := newOpenAICompatStreamGuard(context.Background(), time.Now().Add(30*time.Millisecond), 0)
	reader, writer := io.Pipe()
	defer writer.Close()
	guard.attach(reader)
	defer guard.Close()
	_, err := guard.Read(make([]byte, 1))
	require.ErrorIs(t, err, errOpenAICompatFirstOutputTimeout)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestCompatStreamGuardOutputDisarmsFirstButNotIdle(t *testing.T) {
	_, guard := newOpenAICompatStreamGuard(context.Background(), time.Now().Add(20*time.Millisecond), 60*time.Millisecond)
	reader, writer := io.Pipe()
	defer writer.Close()
	guard.attach(reader)
	defer guard.Close()
	guard.observeOutput()
	_, err := guard.Read(make([]byte, 1))
	require.ErrorIs(t, err, errOpenAICompatStreamIdleTimeout)
}

func TestCompatStreamGuardClientCancelBeforeOutput(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	_, guard := newOpenAICompatStreamGuard(parent, time.Time{}, 0)
	reader, writer := io.Pipe()
	defer writer.Close()
	guard.attach(reader)
	defer guard.Close()
	cancel()
	_, err := guard.Read(make([]byte, 1))
	require.ErrorIs(t, err, context.Canceled)
}

func TestCompatStreamGuardDoesNotTimeDownstreamProcessing(t *testing.T) {
	ctx, guard := newOpenAICompatStreamGuard(context.Background(), time.Time{}, 10*time.Millisecond)
	guard.attach(io.NopCloser(strings.NewReader("ab")))
	defer guard.Close()
	buf := make([]byte, 1)
	_, err := guard.Read(buf)
	require.NoError(t, err)
	guard.observeOutput()
	time.Sleep(30 * time.Millisecond)
	require.NoError(t, ctx.Err())
	_, err = guard.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "b", string(buf))
}

func TestCompatStreamGuardClientCancelAfterOutputPreservesUsageWindow(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx, guard := newOpenAICompatStreamGuard(parent, time.Time{}, 0)
	guard.attach(io.NopCloser(strings.NewReader("usage")))
	defer guard.Close()
	guard.observeOutput()
	cancel()
	require.Eventually(t, func() bool {
		guard.mu.Lock()
		defer guard.mu.Unlock()
		return guard.drain != nil
	}, time.Second, time.Millisecond)
	require.NoError(t, ctx.Err())
	body, err := io.ReadAll(guard)
	require.NoError(t, err)
	require.Equal(t, "usage", string(body))
}
