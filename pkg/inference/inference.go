// Package inference exposes shared inference contracts and event constants for
// downstream product repositories.
package inference

import internal "github.com/sizolity/nobody/internal/inference"

type HealthChecker = internal.HealthChecker
type State = internal.State
type EventEmitter = internal.EventEmitter

const (
	StateConnected     = internal.StateConnected
	StateDegraded      = internal.StateDegraded
	StateReconnecting  = internal.StateReconnecting
	StateDisconnected  = internal.StateDisconnected
	PayloadProviderKey = internal.PayloadProviderKey
)

const (
	EventInferenceCheck            = internal.EventInferenceCheck
	EventInferenceNotReady         = internal.EventInferenceNotReady
	EventInferencePreloadStart     = internal.EventInferencePreloadStart
	EventInferencePreloadDone      = internal.EventInferencePreloadDone
	EventInferenceReconnectAttempt = internal.EventInferenceReconnectAttempt
)
