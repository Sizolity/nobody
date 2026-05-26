// Package beat provides RPG-specific beat pipeline enhancements.
//
// Architecture note: The CLI `beat` command (internal/world/devcli/cmd_beat.go)
// is currently the primary beat execution path. The narrative engine
// (internal/narrative/engine/engine.go) provides an alternative path with its
// own store. Future unification should extract the shared pipeline stages into
// this package, allowing both CLI and engine to use the same tool-aware executor.
//
// Current integration point: AdaptWorld in rpg/bridge/ injects
// RPG rules into the context bundle. The tool loop (toolloop.go) and retry
// utilities (retry.go) are ready for integration when the beat pipeline is
// refactored to use ToolCallingChatModel.
package beat
