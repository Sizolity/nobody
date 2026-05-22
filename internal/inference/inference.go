// Package inference defines shared inference events and health-check contracts.
package inference

import (
	"context"
)

// HealthChecker exposes both a high-level lifecycle primitive (EnsureReady,
// what most callers use) and the low-level primitives (kept separate so
// each stage can be unit-tested independently and so NoOp implementations
// can provide sensible defaults for cloud providers).
type HealthChecker interface {
	// EnsureReady blocks until the provider is ready to accept inference
	// requests or returns an error if readiness cannot be achieved within
	// the provider's internal retry budget.
	EnsureReady(ctx context.Context) error

	// IsModelLoaded reports whether the target model is currently loaded
	// in the provider's runtime (e.g. Ollama /api/ps). Providers without
	// a loaded/unloaded distinction (cloud APIs) always return (true, nil).
	IsModelLoaded(ctx context.Context) (bool, error)

	// PreloadModel instructs the provider to load the target model
	// ahead of the first inference call. No-op for providers that load
	// lazily or have no cold-start distinction.
	PreloadModel(ctx context.Context) error

	// Probe performs a live reachability check and returns the observed
	// State. It does not mutate the checker's own State() cache — use
	// State() for the cached view.
	Probe(ctx context.Context) State

	// State returns the checker's last-observed state. Safe to call
	// concurrently; intended for status displays.
	State() State

	// KeepAlive is provider-specific configuration (Ollama's
	// keep_alive parameter). Providers without this concept SHOULD
	// return "" and document the absence in their package docs.
	KeepAlive() string
}

// State enumerates the reachability of the provider endpoint. Values
// mirror the pre-refactor OllamaConnState so existing dashboards /
// log-search patterns keep working.
type State int

const (
	StateConnected State = iota
	StateDegraded
	StateReconnecting
	StateDisconnected
)

func (s State) String() string {
	switch s {
	case StateConnected:
		return "connected"
	case StateDegraded:
		return "degraded"
	case StateReconnecting:
		return "reconnecting"
	case StateDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// EventEmitter is the signature providers use to push runtime events.
// Callers usually adapt EventLogger.Emit(component="runtime", ...) to this
// shape at wiring time; each provider is free to call emit arbitrarily often,
// including from background goroutines.
//
// eventName MUST be one of the Event* constants declared in events.go.
// severity is one of the standard EventLogger severity levels
// ("info" / "warn" / "error"). payload is the structured event body
// and SHOULD include inference.PayloadProviderKey so consumers can
// filter by provider.
type EventEmitter func(eventName, severity string, payload map[string]any)
