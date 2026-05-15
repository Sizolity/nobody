package llamacpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sizolity/nobody/internal/inference"
)

func TestHealthChecker_Probe_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	h := newHealthChecker(healthCheckerConfig{BaseURL: srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Equal(t, inference.StateConnected, h.Probe(ctx))
	assert.Equal(t, inference.StateConnected, h.State())
}

func TestHealthChecker_Probe_ServerDown(t *testing.T) {
	// 127.0.0.1:0 is unreachable → connection refused → Reconnecting.
	h := newHealthChecker(healthCheckerConfig{BaseURL: "http://127.0.0.1:19999"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	state := h.Probe(ctx)
	assert.Equal(t, inference.StateReconnecting, state)
	assert.Equal(t, inference.StateReconnecting, h.State())
}

func TestHealthChecker_Probe_WarmingUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := newHealthChecker(healthCheckerConfig{BaseURL: srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Equal(t, inference.StateDegraded, h.Probe(ctx))
}

func TestHealthChecker_EnsureReady_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHealthChecker(healthCheckerConfig{BaseURL: srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, h.EnsureReady(ctx))
}

func TestHealthChecker_EnsureReady_ReconnectRecovers(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Fail the first two calls, then start returning 200. This exercises
		// the reconnect loop without needing a real process restart.
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var events []string
	h := newHealthChecker(healthCheckerConfig{
		BaseURL:       srv.URL,
		MaxReconnect:  5,
		ReconnectBase: 5 * time.Millisecond,
		Emit: func(eventName, _ string, _ map[string]any) {
			events = append(events, eventName)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, h.EnsureReady(ctx))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&hits), int32(3))
	assert.Contains(t, events, inference.EventInferenceReconnectAttempt)
}

func TestHealthChecker_EnsureReady_Exhausts(t *testing.T) {
	// Always 503 — reconnect loop should give up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := newHealthChecker(healthCheckerConfig{
		BaseURL:       srv.URL,
		MaxReconnect:  2,
		ReconnectBase: 1 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := h.EnsureReady(ctx)
	assert.Error(t, err)
	assert.Equal(t, inference.StateDisconnected, h.State())
}

func TestHealthChecker_KeepAlive_Empty(t *testing.T) {
	h := newHealthChecker(healthCheckerConfig{BaseURL: "http://localhost:8080"})
	assert.Equal(t, "", h.KeepAlive())
}

func TestHealthChecker_PreloadModel_NoOp(t *testing.T) {
	h := newHealthChecker(healthCheckerConfig{BaseURL: "http://127.0.0.1:19999"})
	assert.NoError(t, h.PreloadModel(context.Background()))
}

func TestLogCheck_HealthyEmitsModelLoaded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var events []map[string]any
	LogCheck(srv.URL, "qwen3.5:latest", func(eventName, severity string, payload map[string]any) {
		events = append(events, map[string]any{
			"eventName": eventName,
			"severity":  severity,
			"payload":   payload,
		})
	})
	if assert.Len(t, events, 1) {
		assert.Equal(t, inference.EventInferenceCheck, events[0]["eventName"])
		assert.Equal(t, "info", events[0]["severity"])
		p := events[0]["payload"].(map[string]any)
		assert.Equal(t, ProviderName, p[inference.PayloadProviderKey])
		assert.Equal(t, "model_loaded", p["event"])
	}
}

func TestLogCheck_WarmingUpEmitsModelUnloaded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	var events []map[string]any
	LogCheck(srv.URL, "qwen3.5:latest", func(eventName, severity string, payload map[string]any) {
		events = append(events, map[string]any{
			"eventName": eventName,
			"severity":  severity,
			"payload":   payload,
		})
	})
	if assert.Len(t, events, 1) {
		assert.Equal(t, "warn", events[0]["severity"])
		p := events[0]["payload"].(map[string]any)
		assert.Equal(t, "model_unloaded", p["event"])
	}
}

func TestFailingChecker_ReportsError(t *testing.T) {
	f := newFailingChecker(assert.AnError)
	err := f.EnsureReady(context.Background())
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, inference.StateDisconnected, f.State())
}

// TestHealthChecker_EnsureReady_RespectsContextCancel verifies the
// reconnect loop bails out promptly with ctx.Err() when the caller
// cancels mid-backoff, instead of running through MaxReconnect attempts.
// Locks in the documented contract that long-running EnsureReady can
// be unwound from harness shutdown without waiting up to 30s per
// remaining backoff window.
func TestHealthChecker_EnsureReady_RespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := newHealthChecker(healthCheckerConfig{
		BaseURL:       srv.URL,
		MaxReconnect:  100, // intentionally large so the test fails if cancel is ignored
		ReconnectBase: 200 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel right after the loop is expected to enter its first backoff.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := h.EnsureReady(ctx)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	// Should unwind well before MaxReconnect * ReconnectBase = 20s.
	assert.Less(t, elapsed, 1*time.Second, "ctx cancel must short-circuit the reconnect loop")
}
