package tools

// EventSink is the minimal structured event contract shared by retained
// logging utilities. It intentionally stays small so future narrative tools can
// emit events without depending on a full harness package.
type EventSink interface {
	Emit(component, event, severity, sessionID string, payload map[string]any)
}
