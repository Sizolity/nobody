// Package director exposes world event proposal sources for downstream
// repositories.
package director

import (
	"context"
	"math/rand"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

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
type ConversationGenerator = internal.ConversationGenerator
type DeepSeekGenerator = internal.DeepSeekGenerator
type DeepSeekGeneratorConfig = internal.DeepSeekGeneratorConfig
type EinoGenerator = internal.EinoGenerator
type EinoGeneratorConfig = internal.EinoGeneratorConfig
type PromptTemplate = internal.PromptTemplate
type PromptTemplateData = internal.PromptTemplateData

const DefaultSystemPrompt = internal.DefaultSystemPrompt
const DefaultMaxRepairAttempts = internal.DefaultMaxRepairAttempts

func ParsePromptTemplate(text string) (*PromptTemplate, error) {
	return internal.ParsePromptTemplate(text)
}

func ParsePromptTemplateWithFormat(text string, ft schema.FormatType) (*PromptTemplate, error) {
	return internal.ParsePromptTemplateWithFormat(text, ft)
}

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

func NewEinoGenerator(m einomodel.BaseChatModel) *EinoGenerator {
	return internal.NewEinoGenerator(m)
}

func NewEinoStreamGenerator(m einomodel.BaseChatModel) *EinoGenerator {
	return internal.NewEinoStreamGenerator(m)
}

func NewEinoChatGenerator(ctx context.Context, cfg EinoGeneratorConfig) (*EinoGenerator, error) {
	return internal.NewEinoChatGenerator(ctx, cfg)
}

func NewProviderGenerator(ctx context.Context, provider, modelName, apiKey string) (*EinoGenerator, error) {
	return internal.NewProviderGenerator(ctx, provider, modelName, apiKey)
}
