package inference

// Event names emitted to logs/runtime.jsonl by HealthChecker
// implementations and by the harness adapter that wraps them (the
// "not ready" event in particular is stamped by harness when a
// CreateChatModel call fails readiness, not by the provider itself).
// Kept in one place so grep / logview / curator scripts have a single
// token set to track, and so adding a new provider cannot silently
// introduce a new event name by copy-paste mistake.
const (
	EventInferenceCheck            = "inference_check"
	EventInferenceNotReady         = "inference_not_ready"
	EventInferencePreloadStart     = "inference_preload_start"
	EventInferencePreloadDone      = "inference_preload_done"
	EventInferenceReconnectAttempt = "inference_reconnect_attempt"
)

// PayloadProviderKey is the JSON key providers MUST use when stamping
// themselves into event payloads. Prefer referencing this constant
// over the string literal so future renames are mechanical.
const PayloadProviderKey = "provider"
