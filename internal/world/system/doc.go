// Package system provides event-builder helpers for world model mutations.
//
// Each system constructs valid WorldEvent values for a specific component
// domain (spatial, inventory, stats, actor). Products call these helpers
// to build events; the events must still be applied through Runtime.ApplyEvent.
//
// Systems do not mutate World directly and are not invoked automatically
// by the runtime step loop.
package system
