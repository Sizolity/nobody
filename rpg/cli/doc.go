// Package cli provides RPG-product-specific CLI commands.
//
// Currently includes:
//   - manage-rule: CRUD for RPG narrative rules
//
// Future:
//   - beat: full beat pipeline with tool-call loop (currently in internal/world/devcli/cmd_beat.go
//     due to deep coupling with devcli helpers; will migrate when beat pipeline is refactored)
package cli
