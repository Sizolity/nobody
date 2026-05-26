package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sizolity/nobody/internal/narrative"
)

func TestLLMDirectorAgentParsesBeatPlan(t *testing.T) {
	t.Parallel()

	plan := BeatPlan{
		BeatID:       "beat_dawn",
		Objective:    "Introduce the merchant.",
		TargetNodeID: "thread_main",
	}
	planJSON, _ := json.Marshal(plan)

	gen := &fakeTextGenerator{response: string(planJSON)}
	agent := NewLLMDirectorAgent(gen)

	bundle := ContextBundle{
		World: narrative.World{ID: "w", Title: "Test"},
		Graph: narrative.StoryGraph{
			CurrentNodeID: "thread_main",
			Nodes:         []narrative.StoryNode{{ID: "thread_main", Type: "quest", Status: "active", Goal: "Find the artifact"}},
		},
	}

	got, err := agent.PlanBeat(context.Background(), bundle)
	if err != nil {
		t.Fatalf("PlanBeat error: %v", err)
	}
	if got.BeatID != "beat_dawn" {
		t.Fatalf("BeatID = %q", got.BeatID)
	}
	if got.TargetNodeID != "thread_main" {
		t.Fatalf("TargetNodeID = %q", got.TargetNodeID)
	}
}

func TestLLMDirectorAgentStripsMarkdown(t *testing.T) {
	t.Parallel()

	planJSON := `{"beat_id":"beat_x","objective":"Test","target_node_id":"n1"}`
	wrapped := "```json\n" + planJSON + "\n```"

	gen := &fakeTextGenerator{response: wrapped}
	agent := NewLLMDirectorAgent(gen)

	bundle := ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
		Graph: narrative.StoryGraph{CurrentNodeID: "n1", Nodes: []narrative.StoryNode{{ID: "n1", Type: "t", Status: "s", Goal: "g"}}},
	}

	got, err := agent.PlanBeat(context.Background(), bundle)
	if err != nil {
		t.Fatalf("PlanBeat error: %v", err)
	}
	if got.BeatID != "beat_x" {
		t.Fatalf("BeatID = %q", got.BeatID)
	}
}

func TestLLMDirectorAgentPropagatesError(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{err: fmt.Errorf("down")}
	agent := NewLLMDirectorAgent(gen)

	_, err := agent.PlanBeat(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLLMDirectorAgentRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: "not json at all"}
	agent := NewLLMDirectorAgent(gen)

	_, err := agent.PlanBeat(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLLMWriterAgentParsesDraft(t *testing.T) {
	t.Parallel()

	draft := narrative.Draft{
		ID:     "draft_dawn",
		BeatID: "beat_dawn",
		Title:  "Dawn Breaks",
		Kind:   "scene",
		Text:   "The sun rose over the valley.",
	}
	draftJSON, _ := json.Marshal(draft)

	gen := &fakeTextGenerator{response: string(draftJSON)}
	agent := NewLLMWriterAgent(gen)

	bundle := ContextBundle{World: narrative.World{ID: "w", Title: "T"}}
	plan := BeatPlan{BeatID: "beat_dawn", Objective: "dawn", TargetNodeID: "n1"}

	got, err := agent.WriteBeat(context.Background(), bundle, plan)
	if err != nil {
		t.Fatalf("WriteBeat error: %v", err)
	}
	if got.ID != "draft_dawn" {
		t.Fatalf("ID = %q", got.ID)
	}
	if got.BeatID != "beat_dawn" {
		t.Fatalf("BeatID = %q", got.BeatID)
	}
	if got.Text != "The sun rose over the valley." {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestLLMWriterAgentPropagatesError(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{err: fmt.Errorf("down")}
	agent := NewLLMWriterAgent(gen)

	_, err := agent.WriteBeat(context.Background(), ContextBundle{}, BeatPlan{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPassContinuityAgentReturnsNoIssues(t *testing.T) {
	t.Parallel()

	agent := PassContinuityAgent{}
	report, err := agent.Check(context.Background(), ContextBundle{}, narrative.Draft{})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(report.Issues))
	}
}

func TestSimpleMemoryAgentCreatesEventFromDraft(t *testing.T) {
	t.Parallel()

	agent := SimpleMemoryAgent{}
	draft := narrative.Draft{
		ID:     "d1",
		BeatID: "beat_test",
		Title:  "Test Scene",
		Kind:   "scene",
		Text:   "Something happened.",
	}

	delta, err := agent.Extract(context.Background(), ContextBundle{}, draft)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if len(delta.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(delta.Events))
	}
	if delta.Events[0].BeatID != "beat_test" {
		t.Fatalf("event BeatID = %q", delta.Events[0].BeatID)
	}
	if delta.Events[0].Summary != "Test Scene" {
		t.Fatalf("event Summary = %q", delta.Events[0].Summary)
	}
}

func TestSimpleStateAgentReturnsGraphUnchanged(t *testing.T) {
	t.Parallel()

	agent := SimpleStateAgent{}
	graph := narrative.StoryGraph{
		CurrentNodeID: "n1",
		Nodes:         []narrative.StoryNode{{ID: "n1", Type: "t", Status: "s", Goal: "g"}},
	}
	bundle := ContextBundle{Graph: graph}

	delta, err := agent.Apply(context.Background(), bundle, BeatPlan{}, MemoryDelta{})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if delta.Graph.CurrentNodeID != "n1" {
		t.Fatalf("CurrentNodeID = %q", delta.Graph.CurrentNodeID)
	}
	if len(delta.Graph.Nodes) != 1 {
		t.Fatalf("Nodes count = %d", len(delta.Graph.Nodes))
	}
}

func TestLLMDirectorAgentCustomPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingTextGenerator{response: `{"beat_id":"b","objective":"o","target_node_id":"n1"}`}
	agent := NewLLMDirectorAgentWithPrompt(gen, "custom prompt")

	bundle := ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
		Graph: narrative.StoryGraph{CurrentNodeID: "n1", Nodes: []narrative.StoryNode{{ID: "n1", Type: "t", Status: "s", Goal: "g"}}},
	}
	agent.PlanBeat(context.Background(), bundle)

	if gen.lastSystem != "custom prompt" {
		t.Fatalf("system prompt = %q, want 'custom prompt'", gen.lastSystem)
	}
}

func TestStripFences(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{"  ```json\n{\"a\":1}\n```  ", `{"a":1}`},
	}
	for _, tc := range cases {
		got := stripFences(tc.in)
		if got != tc.want {
			t.Errorf("stripFences(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- LLM Continuity Agent tests ---

func TestLLMContinuityAgentParsesIssues(t *testing.T) {
	t.Parallel()

	resp := `{"issues":[{"code":"LOCATION_MISMATCH","summary":"Bob is in the tower but should be at market."}]}`
	gen := &fakeTextGenerator{response: resp}
	agent := NewLLMContinuityAgent(gen)

	report, err := agent.Check(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(report.Issues))
	}
	if report.Issues[0].Code != "LOCATION_MISMATCH" {
		t.Errorf("code = %q", report.Issues[0].Code)
	}
}

func TestLLMContinuityAgentNoIssues(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: `{"issues":[]}`}
	agent := NewLLMContinuityAgent(gen)

	report, err := agent.Check(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("issues = %d, want 0", len(report.Issues))
	}
}

func TestLLMContinuityAgentNullIssuesBecomesEmpty(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: `{}`}
	agent := NewLLMContinuityAgent(gen)

	report, err := agent.Check(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if report.Issues == nil {
		t.Fatal("issues should be non-nil empty slice")
	}
}

func TestLLMContinuityAgentPropagatesError(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{err: fmt.Errorf("down")}
	agent := NewLLMContinuityAgent(gen)

	_, err := agent.Check(context.Background(), ContextBundle{}, narrative.Draft{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLLMContinuityAgentRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: "nope"}
	agent := NewLLMContinuityAgent(gen)

	_, err := agent.Check(context.Background(), ContextBundle{}, narrative.Draft{})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLLMContinuityAgentStripsMarkdown(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: "```json\n{\"issues\":[]}\n```"}
	agent := NewLLMContinuityAgent(gen)

	report, err := agent.Check(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("issues = %d", len(report.Issues))
	}
}

func TestLLMContinuityAgentCustomPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingTextGenerator{response: `{"issues":[]}`}
	agent := NewLLMContinuityAgentWithPrompt(gen, "custom check")

	agent.Check(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if gen.lastSystem != "custom check" {
		t.Fatalf("system = %q", gen.lastSystem)
	}
}

// --- LLM Memory Agent tests ---

func TestLLMMemoryAgentParsesEventsAndMemories(t *testing.T) {
	t.Parallel()

	resp := `{
		"events": [{"id":"event_arrival","beat_id":"beat_x","type":"scene","summary":"The merchant arrived.","participant_ids":["char_1"]}],
		"memories": [{"id":"mem_fear","type":"emotion","subject":"char_1","text":"Felt uneasy.","importance":4}]
	}`
	gen := &fakeTextGenerator{response: resp}
	agent := NewLLMMemoryAgent(gen)

	delta, err := agent.Extract(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{BeatID: "beat_x"})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if len(delta.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(delta.Events))
	}
	if delta.Events[0].ID != "event_arrival" {
		t.Errorf("event id = %q", delta.Events[0].ID)
	}
	if delta.Events[0].BeatID != "beat_x" {
		t.Errorf("beat_id = %q", delta.Events[0].BeatID)
	}
	if len(delta.Events[0].ParticipantIDs) != 1 {
		t.Errorf("participants = %d", len(delta.Events[0].ParticipantIDs))
	}
	if len(delta.Memories) != 1 {
		t.Fatalf("memories = %d, want 1", len(delta.Memories))
	}
	if delta.Memories[0].Subject != "char_1" {
		t.Errorf("subject = %q", delta.Memories[0].Subject)
	}
	if delta.Memories[0].Importance != 4 {
		t.Errorf("importance = %d", delta.Memories[0].Importance)
	}
}

func TestLLMMemoryAgentNullFieldsBecomeEmpty(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: `{}`}
	agent := NewLLMMemoryAgent(gen)

	delta, err := agent.Extract(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if delta.Events == nil {
		t.Error("events should be non-nil")
	}
	if delta.Memories == nil {
		t.Error("memories should be non-nil")
	}
}

func TestLLMMemoryAgentPropagatesError(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{err: fmt.Errorf("down")}
	agent := NewLLMMemoryAgent(gen)

	_, err := agent.Extract(context.Background(), ContextBundle{}, narrative.Draft{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLLMMemoryAgentRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: "bad"}
	agent := NewLLMMemoryAgent(gen)

	_, err := agent.Extract(context.Background(), ContextBundle{}, narrative.Draft{})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLLMMemoryAgentCustomPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingTextGenerator{response: `{"events":[],"memories":[]}`}
	agent := NewLLMMemoryAgentWithPrompt(gen, "custom memory")

	agent.Extract(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, narrative.Draft{})
	if gen.lastSystem != "custom memory" {
		t.Fatalf("system = %q", gen.lastSystem)
	}
}

// --- LLM State Agent tests ---

func TestLLMStateAgentParsesGraph(t *testing.T) {
	t.Parallel()

	resp := `{"graph":{"current_node_id":"n2","nodes":[
		{"id":"n1","type":"quest","status":"completed","goal":"Find artifact"},
		{"id":"n2","type":"conflict","status":"active","goal":"Confront the villain"}
	]}}`
	gen := &fakeTextGenerator{response: resp}
	agent := NewLLMStateAgent(gen)

	bundle := ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
		Graph: narrative.StoryGraph{CurrentNodeID: "n1", Nodes: []narrative.StoryNode{
			{ID: "n1", Type: "quest", Status: "active", Goal: "Find artifact"},
		}},
	}
	sd, err := agent.Apply(context.Background(), bundle, BeatPlan{BeatID: "b"}, MemoryDelta{})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if sd.Graph.CurrentNodeID != "n2" {
		t.Errorf("current = %q", sd.Graph.CurrentNodeID)
	}
	if len(sd.Graph.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(sd.Graph.Nodes))
	}
	if sd.Graph.Nodes[0].Status != "completed" {
		t.Errorf("n1 status = %q", sd.Graph.Nodes[0].Status)
	}
}

func TestLLMStateAgentPropagatesError(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{err: fmt.Errorf("down")}
	agent := NewLLMStateAgent(gen)

	_, err := agent.Apply(context.Background(), ContextBundle{}, BeatPlan{}, MemoryDelta{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLLMStateAgentRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	gen := &fakeTextGenerator{response: "nope"}
	agent := NewLLMStateAgent(gen)

	_, err := agent.Apply(context.Background(), ContextBundle{}, BeatPlan{}, MemoryDelta{})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLLMStateAgentStripsMarkdown(t *testing.T) {
	t.Parallel()

	inner := `{"graph":{"current_node_id":"n1","nodes":[{"id":"n1","type":"t","status":"active","goal":"g"}]}}`
	gen := &fakeTextGenerator{response: "```json\n" + inner + "\n```"}
	agent := NewLLMStateAgent(gen)

	sd, err := agent.Apply(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, BeatPlan{}, MemoryDelta{})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if sd.Graph.CurrentNodeID != "n1" {
		t.Errorf("current = %q", sd.Graph.CurrentNodeID)
	}
}

func TestLLMStateAgentCustomPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingTextGenerator{response: `{"graph":{"current_node_id":"n1","nodes":[{"id":"n1","type":"t","status":"s","goal":"g"}]}}`}
	agent := NewLLMStateAgentWithPrompt(gen, "custom state")

	agent.Apply(context.Background(), ContextBundle{
		World: narrative.World{ID: "w", Title: "T"},
	}, BeatPlan{}, MemoryDelta{})
	if gen.lastSystem != "custom state" {
		t.Fatalf("system = %q", gen.lastSystem)
	}
}

// --- test helpers ---

type fakeTextGenerator struct {
	response string
	err      error
}

func (g *fakeTextGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return g.response, g.err
}

type capturingTextGenerator struct {
	response   string
	lastSystem string
	lastUser   string
}

func (g *capturingTextGenerator) Generate(_ context.Context, system, user string) (string, error) {
	g.lastSystem = system
	g.lastUser = user
	return g.response, nil
}
