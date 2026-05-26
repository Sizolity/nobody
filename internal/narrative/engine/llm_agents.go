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

type rewriteInput struct {
	Context ContextBundle     `json:"context"`
	Plan    BeatPlan          `json:"plan"`
	Draft   narrative.Draft   `json:"previous_draft"`
	Issues  []ContinuityIssue `json:"continuity_issues"`
}

const rewriteSystemPrompt = `You are a narrative scene writer revising a draft based on continuity feedback.

You previously wrote a scene that has continuity issues. Rewrite the scene to fix ALL listed issues while preserving the narrative intent and quality.

Return JSON (same schema as the original draft):

{
  "id": "draft_<short_id>_rev",
  "beat_id": "<must match the original beat_id>",
  "title": "Short scene title",
  "kind": "scene",
  "text": "The revised narrative text. 2-5 paragraphs."
}

Rules:
- Fix every continuity issue listed. Do not introduce new issues.
- Keep the same beat_id as the original draft.
- Preserve the original scene's tone, objective, and key narrative beats where possible.
- Return ONLY valid JSON. No markdown, no explanation.`

func (a *LLMWriterAgent) RewriteBeat(ctx context.Context, bundle ContextBundle, plan BeatPlan, draft narrative.Draft, issues []ContinuityIssue) (narrative.Draft, error) {
	userPrompt, err := json.Marshal(rewriteInput{
		Context: bundle, Plan: plan, Draft: draft, Issues: issues,
	})
	if err != nil {
		return narrative.Draft{}, fmt.Errorf("marshal rewrite input: %w", err)
	}
	response, err := a.gen.Generate(ctx, rewriteSystemPrompt, string(userPrompt))
	if err != nil {
		return narrative.Draft{}, fmt.Errorf("llm rewrite: %w", err)
	}
	var revised narrative.Draft
	if err := json.Unmarshal([]byte(stripFences(response)), &revised); err != nil {
		return narrative.Draft{}, fmt.Errorf("parse revised draft: %w", err)
	}
	return revised, nil
}

// --- LLM Continuity Agent ---

const continuitySystemPrompt = `You are a continuity checker for an interactive narrative. Given the world state, characters, locations, recent events, memories, and a draft scene, identify continuity issues.

Return JSON:

{
  "issues": [
    {"code": "LOCATION_MISMATCH", "severity": "critical", "summary": "Character X is described in the tower, but was last seen at the market."},
    {"code": "TONE_MISMATCH", "severity": "warning", "summary": "Draft is slightly more lighthearted than the declared dark tone."}
  ]
}

Issue codes (use the most specific one):
- LOCATION_MISMATCH: character or object in wrong location
- CHARACTER_ABSENT: character not present in the scene's story node but acting in draft
- ITEM_MISSING: character uses an item they don't possess
- RULE_VIOLATION: draft violates an explicit world rule or boundary
- TIMELINE_ERROR: events happen in impossible chronological order
- TONE_MISMATCH: draft tone deviates significantly from the world's declared tone
- OTHER: anything else

Severity levels:
- "critical": factual contradiction that would break world consistency (wrong location, dead character acting, rule violation, impossible timeline)
- "warning": noticeable but non-fatal inconsistency (mild tone shift, minor detail mismatch)
- "info": stylistic or optional improvement suggestion

Rules:
- Only report genuine contradictions with the provided context; do not invent issues.
- If the draft is consistent, return {"issues": []}.
- Return ONLY valid JSON. No markdown, no explanation.`

type LLMContinuityAgent struct {
	gen          TextGenerator
	systemPrompt string
}

func NewLLMContinuityAgent(gen TextGenerator) *LLMContinuityAgent {
	return &LLMContinuityAgent{gen: gen, systemPrompt: continuitySystemPrompt}
}

func NewLLMContinuityAgentWithPrompt(gen TextGenerator, systemPrompt string) *LLMContinuityAgent {
	return &LLMContinuityAgent{gen: gen, systemPrompt: systemPrompt}
}

type continuityInput struct {
	Context ContextBundle   `json:"context"`
	Draft   narrative.Draft `json:"draft"`
}

func (a *LLMContinuityAgent) Check(ctx context.Context, bundle ContextBundle, draft narrative.Draft) (ContinuityReport, error) {
	userPrompt, err := json.Marshal(continuityInput{Context: bundle, Draft: draft})
	if err != nil {
		return ContinuityReport{}, fmt.Errorf("marshal continuity input: %w", err)
	}
	response, err := a.gen.Generate(ctx, a.systemPrompt, string(userPrompt))
	if err != nil {
		return ContinuityReport{}, fmt.Errorf("llm continuity: %w", err)
	}
	var report ContinuityReport
	if err := json.Unmarshal([]byte(stripFences(response)), &report); err != nil {
		return ContinuityReport{}, fmt.Errorf("parse continuity report: %w", err)
	}
	if report.Issues == nil {
		report.Issues = []ContinuityIssue{}
	}
	return report, nil
}

// --- LLM Memory Agent ---

const memorySystemPrompt = `You are a narrative memory extractor. Given the world context and a draft scene, extract the key events and memorable facts.

Return JSON:

{
  "events": [
    {
      "id": "event_<unique_snake_case>",
      "beat_id": "<must match the draft's beat_id>",
      "type": "scene|dialogue|discovery|conflict|resolution",
      "summary": "One-sentence summary of what happened.",
      "participant_ids": ["char_id_1"],
      "effects": {"key": "value"}
    }
  ],
  "memories": [
    {
      "id": "mem_<unique_snake_case>",
      "type": "observation|belief|emotion|relationship|secret",
      "subject": "<character_id or 'world'>",
      "text": "What was learned or felt.",
      "importance": 5
    }
  ]
}

Rules:
- Extract 1-3 events from the scene. Each event should capture a distinct narrative action.
- Extract 0-3 memories. Memories represent lasting impressions, new knowledge, or emotional shifts.
- beat_id in every event MUST match the draft's beat_id.
- importance is 1-10 (1=trivial, 10=world-changing).
- participant_ids should reference character IDs from the context when possible.
- Return ONLY valid JSON. No markdown, no explanation.`

type LLMMemoryAgent struct {
	gen          TextGenerator
	systemPrompt string
}

func NewLLMMemoryAgent(gen TextGenerator) *LLMMemoryAgent {
	return &LLMMemoryAgent{gen: gen, systemPrompt: memorySystemPrompt}
}

func NewLLMMemoryAgentWithPrompt(gen TextGenerator, systemPrompt string) *LLMMemoryAgent {
	return &LLMMemoryAgent{gen: gen, systemPrompt: systemPrompt}
}

type memoryInput struct {
	Context ContextBundle   `json:"context"`
	Draft   narrative.Draft `json:"draft"`
}

func (a *LLMMemoryAgent) Extract(ctx context.Context, bundle ContextBundle, draft narrative.Draft) (MemoryDelta, error) {
	userPrompt, err := json.Marshal(memoryInput{Context: bundle, Draft: draft})
	if err != nil {
		return MemoryDelta{}, fmt.Errorf("marshal memory input: %w", err)
	}
	response, err := a.gen.Generate(ctx, a.systemPrompt, string(userPrompt))
	if err != nil {
		return MemoryDelta{}, fmt.Errorf("llm memory: %w", err)
	}
	var delta MemoryDelta
	if err := json.Unmarshal([]byte(stripFences(response)), &delta); err != nil {
		return MemoryDelta{}, fmt.Errorf("parse memory delta: %w", err)
	}
	if delta.Events == nil {
		delta.Events = []narrative.NarrativeEvent{}
	}
	if delta.Memories == nil {
		delta.Memories = []narrative.Memory{}
	}
	return delta, nil
}

// --- LLM State Agent ---

const stateSystemPrompt = `You are a story graph manager. Given the world context, a beat plan, and the memory delta from the scene, decide how the story graph should change.

The current story graph is in context.graph. You must return an updated graph.

Return JSON:

{
  "graph": {
    "current_node_id": "<id of the node that should be current after this beat>",
    "nodes": [
      {
        "id": "existing_or_new_id",
        "type": "quest|mystery|conflict|exploration|dialogue",
        "status": "active|ready|dormant|completed|failed",
        "goal": "What this story thread is about.",
        "parent_id": "",
        "character_ids": ["char_1"],
        "location_id": "loc_1"
      }
    ]
  }
}

Rules:
- Include ALL existing nodes (you may change their status).
- You may add new nodes if the beat opened a new story thread.
- Mark nodes as "completed" or "failed" if the beat resolved them.
- current_node_id should point to the most active thread after the beat.
- Every node must have id, type, status, and goal.
- Return ONLY valid JSON. No markdown, no explanation.`

type LLMStateAgent struct {
	gen          TextGenerator
	systemPrompt string
}

func NewLLMStateAgent(gen TextGenerator) *LLMStateAgent {
	return &LLMStateAgent{gen: gen, systemPrompt: stateSystemPrompt}
}

func NewLLMStateAgentWithPrompt(gen TextGenerator, systemPrompt string) *LLMStateAgent {
	return &LLMStateAgent{gen: gen, systemPrompt: systemPrompt}
}

type stateInput struct {
	Context ContextBundle `json:"context"`
	Plan    BeatPlan      `json:"plan"`
	Delta   MemoryDelta   `json:"delta"`
}

func (a *LLMStateAgent) Apply(ctx context.Context, bundle ContextBundle, plan BeatPlan, delta MemoryDelta) (StateDelta, error) {
	userPrompt, err := json.Marshal(stateInput{Context: bundle, Plan: plan, Delta: delta})
	if err != nil {
		return StateDelta{}, fmt.Errorf("marshal state input: %w", err)
	}
	response, err := a.gen.Generate(ctx, a.systemPrompt, string(userPrompt))
	if err != nil {
		return StateDelta{}, fmt.Errorf("llm state: %w", err)
	}
	var sd StateDelta
	if err := json.Unmarshal([]byte(stripFences(response)), &sd); err != nil {
		return StateDelta{}, fmt.Errorf("parse state delta: %w", err)
	}
	return sd, nil
}

// --- Pass-through Continuity Agent (stub) ---

type PassContinuityAgent struct{}

func (a PassContinuityAgent) Check(_ context.Context, _ ContextBundle, _ narrative.Draft) (ContinuityReport, error) {
	return ContinuityReport{Issues: []ContinuityIssue{}}, nil
}

// --- Simple Memory Agent (stub) ---

type SimpleMemoryAgent struct{}

func (a SimpleMemoryAgent) Extract(_ context.Context, _ ContextBundle, draft narrative.Draft) (MemoryDelta, error) {
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

// --- Simple State Agent (stub) ---

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
