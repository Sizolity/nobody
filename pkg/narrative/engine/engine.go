// Package engine exposes the product-neutral beat engine and agent contracts
// for downstream Writer and Tavern repositories.
package engine

import (
	internal "github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

type Engine = internal.Engine
type RunBeatInput = internal.RunBeatInput
type RunBeatResult = internal.RunBeatResult
type Agents = internal.Agents
type DirectorAgent = internal.DirectorAgent
type SceneWriterAgent = internal.SceneWriterAgent
type ContinuityAgent = internal.ContinuityAgent
type MemoryAgent = internal.MemoryAgent
type StateAgent = internal.StateAgent
type ContextBundle = internal.ContextBundle
type BeatPlan = internal.BeatPlan
type ContinuityReport = internal.ContinuityReport
type ContinuityIssue = internal.ContinuityIssue
type MemoryDelta = internal.MemoryDelta
type StateDelta = internal.StateDelta

func New(st store.Store, agents Agents) *Engine {
	return internal.New(st, agents)
}
