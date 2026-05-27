package narrator

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/view"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/role"
	rpgrule "github.com/sizolity/nobody/rpg/rule"
	"github.com/sizolity/nobody/rpg/tools"
)

// mockChatModel is a no-op ToolCallingChatModel used by tests that only need
// Narrator to construct (Role / SystemPrompt / Templates / Judge / Tools).
type mockChatModel struct {
	tools []*schema.ToolInfo
}

func (m *mockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: ""}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		w.Send(&schema.Message{Role: schema.Assistant}, nil)
		w.Close()
	}()
	return r, nil
}

func (m *mockChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return &mockChatModel{tools: tools}, nil
}

// mockSuggestModel returns a structured tool call that SuggestActions can parse.
// Reserved for Task 4 — defined here so the file imports compile cleanly.
type mockSuggestModel struct {
	tools []*schema.ToolInfo
}

func (m *mockSuggestModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	args, _ := json.Marshal(map[string]any{
		"options": []role.ActionOption{
			{Label: "调查密室", Type: "investigate"},
			{Label: "与守卫交谈", Type: "social"},
			{Label: "前往遗迹", Type: "explore"},
		},
	})
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "call_suggest_1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "suggest_actions",
				Arguments: string(args),
			},
		}},
	}, nil
}

func (m *mockSuggestModel) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	r, w := schema.Pipe[*schema.Message](1)
	go func() { w.Close() }()
	return r, nil
}

func (m *mockSuggestModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return &mockSuggestModel{tools: tools}, nil
}

func testWorld() model.World {
	combatRule := rpgrule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 0,
		Content: "Attack rolls use d20 + modifier", Source: rpgrule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}
	return model.World{
		ID:   "world-test-01",
		Name: "Crystal Caverns",
		Canon: model.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"mysterious", "dark"},
		},
		Entities: map[model.EntityID]model.Entity{
			"hero-arin": {
				ID: "hero-arin", Type: "character", Name: "Arin",
				Tags: []string{"player", "warrior"},
			},
			"loc-cavern": {
				ID: "loc-cavern", Type: "location", Name: "Ancient Cavern",
				Description: "A deep underground cavern with glowing crystals.",
			},
		},
		Rules: []model.Rule{rpgrule.ToModelRule(combatRule)},
	}
}

func testWorldCtx(w model.World) view.WorldDebugContext {
	return view.WorldDebugView{}.Render(w)
}

func testNarrativeCtx(w model.World) view.NarrativeContext {
	return view.NarrativeView{}.Render(w, view.NarrativeContextRequest{RecentEventLimit: 5})
}

func TestNarrator_Role(t *testing.T) {
	n, err := New(&mockChatModel{})
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Role(); got != "Narrator" {
		t.Errorf("Role() = %q, want %q", got, "Narrator")
	}
}

func TestNarrator_SystemPrompt(t *testing.T) {
	n, _ := New(&mockChatModel{})
	w := testWorld()
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	prompt := n.SystemPrompt(players, role.PromptOptions{
		WorldCtx:     testWorldCtx(w),
		NarrativeCtx: testNarrativeCtx(w),
	})
	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	for _, want := range []string{"Crystal Caverns", "fantasy", "Attack rolls", "Narrator"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestNarrator_SystemPrompt_WithFog(t *testing.T) {
	n, _ := New(&mockChatModel{})
	w := testWorld()
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	prompt := n.SystemPrompt(players, role.PromptOptions{
		WorldCtx:     testWorldCtx(w),
		NarrativeCtx: testNarrativeCtx(w),
		FogEnabled:   true,
	})
	if !strings.Contains(prompt, "Discovery Protocol") {
		t.Error("fog enabled: should contain discovery protocol")
	}
}

// toolNames extracts the set of tool names from an InvokableTool slice, failing
// the test on any Info() error rather than silently swallowing it.
func toolNames(t *testing.T, tls []tool.InvokableTool) map[string]bool {
	t.Helper()
	names := make(map[string]bool, len(tls))
	for i, tl := range tls {
		info, err := tl.Info(context.Background())
		if err != nil {
			t.Fatalf("tool[%d].Info(): %v", i, err)
		}
		names[info.Name] = true
	}
	return names
}

func TestNarrator_Tools_BaseOnly(t *testing.T) {
	n, _ := New(&mockChatModel{})
	w := testWorld()
	w.Rules = nil // no rules → lookup_rules not disclosed
	// testWorld() entities have no State map and no StatsComponent → no update_state
	tc := &tools.ToolContext{World: w, Rng: rand.New(rand.NewPCG(1, 2))}
	invokable, err := n.Tools(tc)
	if err != nil {
		t.Fatalf("Tools(): %v", err)
	}
	if got, want := len(invokable), 2; got != want {
		t.Errorf("len(tools) = %d, want %d (get_entity_state + roll only)", got, want)
	}
	names := toolNames(t, invokable)
	for _, want := range []string{"get_entity_state", "roll"} {
		if !names[want] {
			t.Errorf("expected %q to be disclosed, got %v", want, names)
		}
	}
	for _, forbidden := range []string{"lookup_rules", "update_state", "explore_knowledge"} {
		if names[forbidden] {
			t.Errorf("%q must NOT be disclosed under base-only conditions, got %v", forbidden, names)
		}
	}
}

func TestNarrator_Tools_WithFog(t *testing.T) {
	n, _ := New(&mockChatModel{})
	disclosure := fog.DisclosureState{}
	tc := &tools.ToolContext{
		World:      testWorld(), // has 1 rule, no mutable entities
		Rng:        rand.New(rand.NewPCG(1, 2)),
		Disclosure: &disclosure,
	}
	invokable, err := n.Tools(tc)
	if err != nil {
		t.Fatalf("Tools(): %v", err)
	}
	names := toolNames(t, invokable)
	// Expected disclosed set: get_entity_state + roll (always) + lookup_rules
	// (1 rule present) + explore_knowledge (fog enabled). update_state stays
	// hidden because testWorld() entities carry no mutable state.
	for _, want := range []string{"get_entity_state", "roll", "lookup_rules", "explore_knowledge"} {
		if !names[want] {
			t.Errorf("expected %q to be disclosed, got %v", want, names)
		}
	}
	if names["update_state"] {
		t.Errorf("update_state must NOT be disclosed without mutable entities, got %v", names)
	}
}

func TestNarrator_Judge(t *testing.T) {
	n, _ := New(&mockChatModel{})
	action := role.PlayerAction{PlayerID: "p1", Content: "I open the door."}
	j, err := n.Judge(context.Background(), action, testWorld())
	if err != nil {
		t.Fatalf("Judge(): %v", err)
	}
	if j.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", j.Outcome, "success")
	}
}

func TestNarrator_SuggestActions(t *testing.T) {
	n, _ := New(&mockSuggestModel{})
	players := []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Player1"},
	}
	narrative := "The ancient door creaks open, revealing a dimly lit chamber."
	choices, err := n.SuggestActions(context.Background(), testWorld(), players, narrative)
	if err != nil {
		t.Fatalf("SuggestActions(): %v", err)
	}
	if got := len(choices.Options); got < 2 || got > 4 {
		t.Errorf("expected 2-4 options, got %d", got)
	}
	for i, opt := range choices.Options {
		if opt.Label == "" {
			t.Errorf("option[%d].Label is empty", i)
		}
		if opt.Type == "" {
			t.Errorf("option[%d].Type is empty", i)
		}
	}
}

func TestNarrator_Templates(t *testing.T) {
	n, _ := New(&mockChatModel{})
	templates := n.Templates()
	if got, want := len(templates), 4; got != want {
		t.Errorf("len(templates) = %d, want %d", got, want)
	}
	names := map[string]bool{}
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	for _, want := range []string{"fantasy", "mystery", "scifi", "modern"} {
		if !names[want] {
			t.Errorf("missing template %q", want)
		}
	}
}

// TestNarrator_ImplementsGM is a compile-time assertion that *Narrator
// satisfies role.GM (all four sub-interfaces). If any GM method is missing
// or has a mismatched signature, this test file will fail to compile.
func TestNarrator_ImplementsGM(t *testing.T) {
	var _ role.GM = (*Narrator)(nil)
}
