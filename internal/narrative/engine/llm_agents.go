package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/narrative"
)

// TextGenerator abstracts an LLM inference call. Compatible with the
// world director TextGenerator interface.
type TextGenerator interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// --- LLM Director Agent ---

const directorSystemPrompt = `You are a narrative director planning the next beat of an interactive story.

Given the current world state, story graph, characters, and recent events, produce a beat plan as JSON:

{
  "beat_id": "beat_<short_snake_case_descriptor>",
  "objective": "One sentence describing what this beat should accomplish narratively.",
  "target_node_id": "<id of an existing story node from the graph>"
}

Rules:
- beat_id must be unique and descriptive (e.g. "beat_merchant_arrives", "beat_confrontation").
- objective should advance the story meaningfully, considering user input if present.
- target_node_id MUST reference an existing node from the story graph.
- Return ONLY valid JSON. No markdown, no explanation.`

type LLMDirectorAgent struct {
	gen          TextGenerator
	systemPrompt string
}

func NewLLMDirectorAgent(gen TextGenerator) *LLMDirectorAgent {
	return &LLMDirectorAgent{gen: gen, systemPrompt: directorSystemPrompt}
}

func NewLLMDirectorAgentWithPrompt(gen TextGenerator, systemPrompt string) *LLMDirectorAgent {
	return &LLMDirectorAgent{gen: gen, systemPrompt: systemPrompt}
}

func (a *LLMDirectorAgent) PlanBeat(ctx context.Context, bundle ContextBundle) (BeatPlan, error) {
	userPrompt, err := json.Marshal(bundle)
	if err != nil {
		return BeatPlan{}, fmt.Errorf("marshal context: %w", err)
	}
	response, err := a.gen.Generate(ctx, a.systemPrompt, string(userPrompt))
	if err != nil {
		return BeatPlan{}, fmt.Errorf("llm director: %w", err)
	}
	var plan BeatPlan
	if err := json.Unmarshal([]byte(stripFences(response)), &plan); err != nil {
		return BeatPlan{}, fmt.Errorf("parse beat plan: %w", err)
	}
	return plan, nil
}

// --- LLM Writer Agent ---

const writerSystemPrompt = `You are a narrative scene writer. Given the world state and a beat plan, write a short narrative scene.

Return JSON:

{
  "id": "draft_<short_id>",
  "beat_id": "<must match the beat plan's beat_id>",
  "title": "Short scene title",
  "kind": "scene",
  "text": "The narrative text of the scene. 2-5 paragraphs."
}

Rules:
- The text should be vivid prose that advances the beat plan's objective.
- Stay consistent with the world's tone, genre, and rules.
- Use character names and location details from the context.
- beat_id must exactly match the plan's beat_id.
- Return ONLY valid JSON. No markdown, no explanation.`

type LLMWriterAgent struct {
	gen          TextGenerator
	systemPrompt string
}

func NewLLMWriterAgent(gen TextGenerator) *LLMWriterAgent {
	return &LLMWriterAgent{gen: gen, systemPrompt: writerSystemPrompt}
}

func NewLLMWriterAgentWithPrompt(gen TextGenerator, systemPrompt string) *LLMWriterAgent {
	return &LLMWriterAgent{gen: gen, systemPrompt: systemPrompt}
}

type writerInput struct {
	Context ContextBundle `json:"context"`
	Plan    BeatPlan      `json:"plan"`
}

func (a *LLMWriterAgent) WriteBeat(ctx context.Context, bundle ContextBundle, plan BeatPlan) (narrative.Draft, error) {
	userPrompt, err := json.Marshal(writerInput{Context: bundle, Plan: plan})
	if err != nil {
		return narrative.Draft{}, fmt.Errorf("marshal writer input: %w", err)
	}
	response, err := a.gen.Generate(ctx, a.systemPrompt, string(userPrompt))
	if err != nil {
		return narrative.Draft{}, fmt.Errorf("llm writer: %w", err)
	}
	var draft narrative.Draft
	if err := json.Unmarshal([]byte(stripFences(response)), &draft); err != nil {
		return narrative.Draft{}, fmt.Errorf("parse draft: %w", err)
	}
	return draft, nil
}

// --- Pass-through Continuity Agent ---

type PassContinuityAgent struct{}

func (a PassContinuityAgent) Check(_ context.Context, _ ContextBundle, _ narrative.Draft) (ContinuityReport, error) {
	return ContinuityReport{Issues: []ContinuityIssue{}}, nil
}

// --- Simple Memory Agent ---

// SimpleMemoryAgent creates a single narrative event per beat from the draft,
// without LLM inference.
type SimpleMemoryAgent struct{}

func (a SimpleMemoryAgent) Extract(_ context.Context, bundle ContextBundle, draft narrative.Draft) (MemoryDelta, error) {
	event := narrative.NarrativeEvent{
		ID:      "event_" + draft.BeatID,
		BeatID:  draft.BeatID,
		Type:    "scene",
		Summary: draft.Title,
	}
	return MemoryDelta{
		Events:   []narrative.NarrativeEvent{event},
		Memories: []narrative.Memory{},
	}, nil
}

// --- Simple State Agent ---

// SimpleStateAgent returns the graph unchanged. It satisfies the StateAgent
// interface without requiring LLM inference.
type SimpleStateAgent struct{}

func (a SimpleStateAgent) Apply(_ context.Context, bundle ContextBundle, _ BeatPlan, _ MemoryDelta) (StateDelta, error) {
	return StateDelta{Graph: bundle.Graph}, nil
}

// --- Helpers ---

func stripFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	idx := strings.Index(trimmed, "\n")
	if idx < 0 {
		return trimmed
	}
	inner := trimmed[idx+1:]
	if last := strings.LastIndex(inner, "```"); last >= 0 {
		inner = inner[:last]
	}
	return strings.TrimSpace(inner)
}
