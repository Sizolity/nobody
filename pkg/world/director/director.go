// Package director exposes world event proposal sources for downstream
// repositories.
package director

import (
	"math/rand"

	internal "github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

type Director = internal.Director
type Context = internal.Context

type ScriptDirector = internal.ScriptDirector
type ReconcileDirector = internal.ReconcileDirector
type ReconcileCase = internal.ReconcileCase
type EventTableDirector = internal.EventTableDirector
type EventTableEntry = internal.EventTableEntry
type RandomDirector = internal.RandomDirector
type LLMDirector = internal.LLMDirector
type LLMDirectorConfig = internal.LLMDirectorConfig
type TextGenerator = internal.TextGenerator
type DeepSeekGenerator = internal.DeepSeekGenerator
type DeepSeekGeneratorConfig = internal.DeepSeekGeneratorConfig

func NewScriptDirector(id string, events []model.WorldEvent) ScriptDirector {
	return internal.NewScriptDirector(id, events)
}

func NewReconcileDirector(id string, cases []ReconcileCase) ReconcileDirector {
	return internal.NewReconcileDirector(id, cases)
}

func NewEventTableDirector(id string, entries []EventTableEntry) EventTableDirector {
	return internal.NewEventTableDirector(id, entries)
}

func NewRandomDirector(id string, entries []EventTableEntry, rng *rand.Rand) RandomDirector {
	return internal.NewRandomDirector(id, entries, rng)
}

func NewLLMDirector(id string, config LLMDirectorConfig) LLMDirector {
	return internal.NewLLMDirector(id, config)
}

func NewDeepSeekGenerator(config DeepSeekGeneratorConfig) *DeepSeekGenerator {
	return internal.NewDeepSeekGenerator(config)
}
