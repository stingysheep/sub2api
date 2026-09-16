package service

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	errOpenAICompatFirstOutputTimeout = errors.New("upstream first output timeout")
	errOpenAICompatStreamIdleTimeout  = errors.New("upstream stream data interval timeout")
)

// The header and body phases share one first-output deadline. Closing the body
// also interrupts synchronous Scanner reads, including detached CC requests.
type openAICompatStreamGuard struct {
	mu           sync.Mutex
	cancel       context.CancelFunc
	body         io.ReadCloser
	bodyClose    sync.Once
	err          error
	closed       bool
	output       bool
	reading      bool
	first        *time.Timer
	idle         *time.Timer
	drain        *time.Timer
	idleTimeout  time.Duration
	idleDeadline time.Time
	stopParent   func() bool
}

func newOpenAICompatStreamGuard(parent context.Context, deadline time.Time, idleTimeout time.Duration) (context.Context, *openAICompatStreamGuard) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	g := &openAICompatStreamGuard{cancel: cancel, idleTimeout: idleTimeout}
	if err := parent.Err(); err != nil {
		g.err = err
		cancel()
		return ctx, g
	}
	if !deadline.IsZero() {
		g.first = time.AfterFunc(time.Until(deadline), func() { g.expire(errOpenAICompatFirstOutputTimeout, true) })
	}
	g.stopParent = context.AfterFunc(parent, func() {
		g.mu.Lock()
		if g.closed || g.err != nil {
			g.mu.Unlock()
			return
		}
		if g.output {
			// Keep collecting final usage, but never leave an orphaned stream.
			g.drain = time.AfterFunc(30*time.Second, func() { g.expire(context.Canceled, false) })
			g.mu.Unlock()
			return
		}
		g.err = context.Canceled
		g.mu.Unlock()
		g.cancel()
		g.closeBody()
	})
	return ctx, g
}

func (g *openAICompatStreamGuard) attach(body io.ReadCloser) {
	g.mu.Lock()
	g.body = body
	failed := g.err != nil || g.closed
	g.mu.Unlock()
	if failed {
		g.closeBody()
	}
}

func (g *openAICompatStreamGuard) expireIdle() {
	g.mu.Lock()
	if g.closed || g.err != nil || !g.reading {
		g.mu.Unlock()
		return
	}
	if remaining := time.Until(g.idleDeadline); remaining > 0 {
		g.idle.Reset(remaining)
		g.mu.Unlock()
		return
	}
	g.err = errOpenAICompatStreamIdleTimeout
	g.mu.Unlock()
	g.cancel()
	g.closeBody()
}

func (g *openAICompatStreamGuard) expire(err error, beforeOutput bool) {
	g.mu.Lock()
	if g.closed || g.err != nil || (beforeOutput && g.output) {
		g.mu.Unlock()
		return
	}
	g.err = err
	g.mu.Unlock()
	g.cancel()
	g.closeBody()
}

func (g *openAICompatStreamGuard) closeBody() {
	g.mu.Lock()
	body := g.body
	g.mu.Unlock()
	if body != nil {
		g.bodyClose.Do(func() { _ = body.Close() })
	}
}

func (g *openAICompatStreamGuard) Err() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.err
}

func (g *openAICompatStreamGuard) observeOutput() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.output = true
	if g.first != nil {
		g.first.Stop()
	}
}

func (g *openAICompatStreamGuard) Read(p []byte) (int, error) {
	g.mu.Lock()
	if g.err != nil {
		err := g.err
		g.mu.Unlock()
		return 0, err
	}
	// Downstream Write/Flush backpressure is not an upstream read timeout.
	g.reading = true
	if !g.closed && g.idleTimeout > 0 {
		g.idleDeadline = time.Now().Add(g.idleTimeout)
		if g.idle == nil {
			g.idle = time.AfterFunc(g.idleTimeout, g.expireIdle)
		} else {
			g.idle.Reset(g.idleTimeout)
		}
	}
	g.mu.Unlock()
	n, err := g.body.Read(p)
	g.mu.Lock()
	g.reading = false
	if g.idle != nil {
		g.idle.Stop()
	}
	if g.err != nil {
		err = g.err
	}
	g.mu.Unlock()
	return n, err
}

func (g *openAICompatStreamGuard) Close() error {
	g.mu.Lock()
	g.closed = true
	for _, timer := range []*time.Timer{g.first, g.idle, g.drain} {
		if timer != nil {
			timer.Stop()
		}
	}
	g.mu.Unlock()
	if g.stopParent != nil {
		g.stopParent()
	}
	g.cancel()
	g.closeBody()
	return nil
}

func observeOpenAICompatOutput(body io.ReadCloser) {
	if guard, ok := body.(*openAICompatStreamGuard); ok {
		guard.observeOutput()
	}
}
