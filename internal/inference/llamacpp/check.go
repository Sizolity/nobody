package llamacpp

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/sizolity/nobody/internal/inference"
)

// LogCheck performs a one-shot readiness probe at harness startup and
// emits a structured inference_check event describing the outcome. It
// is the llamacpp counterpart to ollama.LogCheck and is exported for
// symmetry with the ollama subpackage (neither is called from outside
// its own factory today).
//
// Unlike Ollama, llama-server does not expose a per-model ps endpoint,
// so the payload is limited to reachability and HTTP status — there is
// no "VRAM on GPU %" read-out to emit. If llama.cpp grows a richer
// introspection endpoint in the future (e.g. /slots metadata beyond
// the allocation state), this is the place to extend.
func LogCheck(baseURL, modelName string, emit inference.EventEmitter) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		emitCheck(emit, inference.EventInferenceCheck, "warn", map[string]any{
			inference.PayloadProviderKey: ProviderName,
			"event":                      "query_failed",
			"model":                      modelName,
			"error":                      err.Error(),
		})
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		emitCheck(emit, inference.EventInferenceCheck, "warn", map[string]any{
			inference.PayloadProviderKey: ProviderName,
			"event":                      "query_failed",
			"model":                      modelName,
			"error":                      err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		emitCheck(emit, inference.EventInferenceCheck, "info", map[string]any{
			inference.PayloadProviderKey: ProviderName,
			"event":                      "model_loaded",
			"model":                      modelName,
			"status":                     resp.StatusCode,
		})
		return
	}

	// Non-2xx is surfaced as "model_unloaded" to match the ollama event
	// vocabulary; llama-server's 503 during slot warmup flows through
	// this branch and the reconnect loop picks it up from there.
	emitCheck(emit, inference.EventInferenceCheck, "warn", map[string]any{
		inference.PayloadProviderKey: ProviderName,
		"event":                      "model_unloaded",
		"model":                      modelName,
		"status":                     resp.StatusCode,
	})
}

func emitCheck(emit inference.EventEmitter, eventName, severity string, payload map[string]any) {
	if emit != nil {
		emit(eventName, severity, payload)
		return
	}
	event, _ := payload["event"].(string)
	if event == "" {
		event = "unknown"
	}
	log.Printf("[llamacpp-check] severity=%s event=%s payload=%v", severity, event, payload)
}
