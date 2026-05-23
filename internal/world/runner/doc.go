// Package runner orchestrates persistent world runtime steps.
//
// It connects snapshot-capable stores to the pure event application runtime:
// load a world snapshot, apply one event, then save the resulting snapshot.
package runner
