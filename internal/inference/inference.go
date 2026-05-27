// Package inference defines provider-neutral inference contracts.
// Concrete implementations live in the product layer.
package inference

import (
	"context"
)

// HealthChecker abstracts readiness and reachability probing for any
// inference provider. Products supply concrete implementations;
// the world framework depends only on this interface.
type HealthChecker interface {
	EnsureReady(ctx context.Context) error
	IsModelLoaded(ctx context.Context) (bool, error)
	PreloadModel(ctx context.Context) error
	Probe(ctx context.Context) State
	State() State
	KeepAlive() string
}

// State enumerates the reachability of the provider endpoint.
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
// eventName MUST be one of the Event* constants declared in events.go.
type EventEmitter func(eventName, severity string, payload map[string]any)
