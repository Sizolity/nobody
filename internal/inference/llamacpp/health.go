package llamacpp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sizolity/nobody/internal/inference"
)

// healthCheckerConfig captures the post-default values used to drive
// the probe loop. Fields mirror the options the operator can override
// via provider_opts.llamacpp.*; newHealthChecker applies defaults.
type healthCheckerConfig struct {
	BaseURL       string
	ModelName     string
	ProbePath     string        // default "/health"
	MaxReconnect  int           // default 5
	ReconnectBase time.Duration // default 1s
	Emit          inference.EventEmitter
	HTTPClient    *http.Client // nil → http.DefaultClient (tests inject stubs)
}

// healthChecker is the runtime-facing llamacpp HealthChecker. Compared
// to the ollama variant it is deliberately lean because llama-server
// loads the model at process start: there is no cold-start distinction
// to track, no keep-alive to refresh, and PreloadModel is a no-op.
type healthChecker struct {
	cfg   healthCheckerConfig
	state inference.State
	mu    sync.RWMutex
}

func newHealthChecker(cfg healthCheckerConfig) *healthChecker {
	if cfg.ProbePath == "" {
		cfg.ProbePath = "/health"
	}
	if cfg.MaxReconnect <= 0 {
		cfg.MaxReconnect = 5
	}
	if cfg.ReconnectBase <= 0 {
		cfg.ReconnectBase = 1 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &healthChecker{
		cfg:   cfg,
		state: inference.StateConnected,
	}
}

func (h *healthChecker) State() inference.State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

func (h *healthChecker) setState(s inference.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = s
}

// KeepAlive returns "" because llama-server has no keep-alive concept;
// inference.HealthChecker.KeepAlive is documented to return "" when a
// provider doesn't have the semantic, which callers such as the think
// budget allocator already handle.
func (h *healthChecker) KeepAlive() string { return "" }

// IsModelLoaded asks llama-server whether it is up. Because llama-server
// only ever serves a single model per process, a successful /health hit
// means the configured model is loaded; there is no per-model ps list
// to consult (unlike Ollama's /api/ps).
func (h *healthChecker) IsModelLoaded(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.cfg.BaseURL+h.cfg.ProbePath, nil)
	if err != nil {
		return false, err
	}
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}
	// llama-server returns 503 during --parallel slot warmup and 500 on
	// internal errors; surface non-2xx as "not loaded" rather than
	// transport failure so Probe reports Degraded (local recovery path)
	// instead of Reconnecting (loop with backoff).
	return false, nil
}

// PreloadModel is a no-op. llama-server loads the model synchronously
// during `./llama-server` startup, so by the time /health returns 200
// the model is already in memory. NoOp here also satisfies the
// inference.HealthChecker contract for providers without cold-start
// semantics.
func (h *healthChecker) PreloadModel(context.Context) error { return nil }

func (h *healthChecker) Probe(ctx context.Context) inference.State {
	loaded, err := h.IsModelLoaded(ctx)
	if err != nil {
		if isConnRefused(err) {
			h.setState(inference.StateReconnecting)
			return inference.StateReconnecting
		}
		h.setState(inference.StateDegraded)
		return inference.StateDegraded
	}
	if !loaded {
		h.setState(inference.StateDegraded)
		return inference.StateDegraded
	}
	h.setState(inference.StateConnected)
	return inference.StateConnected
}

func (h *healthChecker) EnsureReady(ctx context.Context) error {
	state := h.Probe(ctx)
	switch state {
	case inference.StateConnected:
		return nil
	case inference.StateDegraded:
		// llama-server has no preload handle; the most we can do is
		// retry until /health flips. Fall through to the same backoff
		// loop used for connection-refused so the operator sees a
		// uniform reconnect story in the runtime log.
		return h.reconnectLoop(ctx)
	case inference.StateReconnecting:
		return h.reconnectLoop(ctx)
	default:
		return fmt.Errorf("llamacpp disconnected: recovery exhausted")
	}
}

func (h *healthChecker) reconnectLoop(ctx context.Context) error {
	backoff := h.cfg.ReconnectBase
	for attempt := 1; attempt <= h.cfg.MaxReconnect; attempt++ {
		h.emit(inference.EventInferenceReconnectAttempt, "warn", map[string]any{
			inference.PayloadProviderKey: ProviderName,
			"event":                      "reconnect_attempt",
			"attempt":                    attempt,
			"backoff":                    backoff.String(),
		})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if h.Probe(ctx) == inference.StateConnected {
			return nil
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
	h.setState(inference.StateDisconnected)
	return fmt.Errorf("llamacpp reconnect failed after %d attempts", h.cfg.MaxReconnect)
}

func (h *healthChecker) emit(eventName, severity string, payload map[string]any) {
	if h.cfg.Emit != nil {
		h.cfg.Emit(eventName, severity, payload)
	}
}

// isConnRefused recognises the two transport errors that indicate the
// llama-server process itself is absent (rather than an in-process
// error). Kept string-matched like the ollama implementation because
// net.OpError.Error() wraps os-dependent messages and the stdlib does
// not expose a sentinel for these two cases as of Go 1.25.
func isConnRefused(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host")
}

// failingChecker is returned when factory.NewHealthChecker encounters
// an unrecoverable config error (e.g. provider_opts.llamacpp.mode set
// to an unsupported value). EnsureReady reports the captured error so
// runtime initialization rejects the run up-front rather than on the first
// inference call.
type failingChecker struct{ err error }

func newFailingChecker(err error) *failingChecker { return &failingChecker{err: err} }

func (f *failingChecker) EnsureReady(context.Context) error           { return f.err }
func (f *failingChecker) IsModelLoaded(context.Context) (bool, error) { return false, f.err }
func (f *failingChecker) PreloadModel(context.Context) error          { return f.err }
func (f *failingChecker) Probe(context.Context) inference.State       { return inference.StateDisconnected }
func (f *failingChecker) State() inference.State                      { return inference.StateDisconnected }
func (f *failingChecker) KeepAlive() string                           { return "" }

// Compile-time assertions.
var (
	_ inference.HealthChecker = (*healthChecker)(nil)
	_ inference.HealthChecker = (*failingChecker)(nil)
)
