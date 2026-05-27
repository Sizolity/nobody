package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/rule"
)

// mockChatModel simulates a ToolCallingChatModel that:
// 1st call: returns a tool call (roll d20)
// 2nd call: returns the final narrative text
type mockChatModel struct {
	callCount int
	tools     []*schema.ToolInfo
}

func (m *mockChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.callCount++

	if m.callCount == 1 && len(m.tools) > 0 {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "roll",
						Arguments: `{"sides":20,"count":1,"modifier":2}`,
					},
				},
			},
		}, nil
	}

	return &schema.Message{
		Role:    schema.Assistant,
		Content: "The ancient door creaks open, revealing a dimly lit chamber. Your torch flickers as cold air rushes past. In the center, a stone pedestal holds a glowing crystal.",
	}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "Streaming not used in test.",
	}
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		w.Send(msg, nil)
		w.Close()
	}()
	return r, nil
}

func (m *mockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &mockChatModel{tools: tools}, nil
}

func setupTestWorld(t *testing.T) (string, worldmodel.World) {
	t.Helper()
	dir := t.TempDir()

	combatRule := rule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 0,
		Content: "Attack rolls use d20 + modifier", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}

	world := worldmodel.World{
		ID:   "world-test-01",
		Name: "Crystal Caverns",
		Canon: worldmodel.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"mysterious", "dark"},
		},
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-arin": {
				ID: "hero-arin", Type: "character", Name: "Arin",
				Tags: []string{"player", "warrior"},
				State: map[string]worldmodel.Value{
					"hp":       {Kind: worldmodel.ValueKindNumber, Raw: float64(25)},
					"strength": {Kind: worldmodel.ValueKindNumber, Raw: float64(14)},
				},
			},
			"loc-cavern": {
				ID: "loc-cavern", Type: "location", Name: "Ancient Cavern",
				Description: "A deep underground cavern with glowing crystals.",
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-explore", Kind: worldmodel.ThreadKindQuest,
				Title:  "Explore the Crystal Caverns",
				Status: worldmodel.ThreadStatusActive,
			},
		},
		Rules: []worldmodel.Rule{
			rule.ToModelRule(combatRule),
		},
		Clock: worldmodel.WorldClock{Sequence: 5},
	}

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("save test world: %v", err)
	}

	return dir, world
}

func TestRunBeat_FullPipeline(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		WorkspacePath: dir,
		ChatModel:     &mockChatModel{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID:   "world-test-01",
		UserInput: "I push open the ancient door.",
	})
	if err != nil {
		t.Fatalf("RunBeat: %v", err)
	}

	if output.Narrative == "" {
		t.Error("expected non-empty narrative")
	}
	if output.World.Clock.Sequence != 6 {
		t.Errorf("expected clock sequence 6, got %d", output.World.Clock.Sequence)
	}

	// Verify world was persisted
	loaded, err := sess.LoadWorld(context.Background(), "world-test-01")
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if loaded.Clock.Sequence != 6 {
		t.Errorf("persisted clock sequence: expected 6, got %d", loaded.Clock.Sequence)
	}
}

func TestRunBeat_WithToolCalls(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		WorkspacePath: dir,
		ChatModel:     &mockChatModel{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID:   "world-test-01",
		UserInput: "I attack the stone golem.",
	})
	if err != nil {
		t.Fatalf("RunBeat: %v", err)
	}

	if output.Narrative == "" {
		t.Error("expected non-empty narrative")
	}
	t.Logf("Narrative: %s", output.Narrative)
	t.Logf("Effects: %d", len(output.ToolEffects))
}

func TestNew_Validation(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}

	_, err = New(Config{WorkspacePath: "/tmp/test"})
	if err == nil {
		t.Fatal("expected error for missing ChatModel")
	}
}

func TestApplyEffects(t *testing.T) {
	world := worldmodel.World{
		ID:   "w1",
		Name: "Test",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"ent-1": {
				ID: "ent-1", Type: "character", Name: "Hero",
				State: map[string]worldmodel.Value{
					"hp": {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
				},
			},
		},
	}

	effects := []worldmodel.Effect{
		{
			Kind:     worldmodel.EffectUpdateEntityState,
			TargetID: "ent-1",
			Payload: map[string]worldmodel.Value{
				"hp": {Kind: worldmodel.ValueKindNumber, Raw: float64(15)},
			},
		},
	}

	result := applyEffects(world, effects)
	ent := result.Entities["ent-1"]
	hp := ent.State["hp"]
	if hp.Raw != float64(15) {
		t.Errorf("expected hp=15, got %v", hp.Raw)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	_, world := setupTestWorld(t)
	prompt := buildSystemPrompt(world, 5, false)
	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if !containsStr(prompt, "Crystal Caverns") {
		t.Error("prompt should contain world name")
	}
	if !containsStr(prompt, "fantasy") {
		t.Error("prompt should contain genre")
	}
	if !containsStr(prompt, "Attack rolls") {
		t.Error("prompt should contain rules")
	}
	if containsStr(prompt, "Discovery Protocol") {
		t.Error("fog disabled: should NOT contain discovery protocol")
	}
}

func TestBuildSystemPrompt_WithFog(t *testing.T) {
	_, world := setupTestWorld(t)
	prompt := buildSystemPrompt(world, 5, true)
	if !containsStr(prompt, "Discovery Protocol") {
		t.Error("fog enabled: should contain discovery protocol")
	}
	if !containsStr(prompt, "explore_knowledge") {
		t.Error("fog enabled: should reference explore_knowledge tool")
	}
}

func TestWorldPersistence(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(dir)

	world := worldmodel.World{
		ID:   "persist-test",
		Name: "Persistence Test",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"e1": {ID: "e1", Name: "Entity1", Type: "character"},
		},
	}
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("save: %v", err)
	}

	files, _ := os.ReadDir(filepath.Join(dir, "worlds"))
	if len(files) == 0 {
		t.Fatal("expected world file to be created")
	}

	loaded, err := fs.LoadSnapshot(context.Background(), "persist-test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Name != "Persistence Test" {
		t.Errorf("expected name 'Persistence Test', got %q", loaded.Name)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && json.Valid([]byte("null")) && // compile check
		findSubstr(s, sub)
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// mockExploreModel returns an explore_knowledge tool call followed by narrative.
type mockExploreModel struct {
	callCount int
	tools     []*schema.ToolInfo
}

func (m *mockExploreModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.callCount++
	if m.callCount == 1 && len(m.tools) > 0 {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_fog",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "explore_knowledge",
						Arguments: `{"target_id":"loc-cavern","level":"explored"}`,
					},
				},
			},
		}, nil
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: "You discover the Ancient Cavern in full detail.",
	}, nil
}

func (m *mockExploreModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg := &schema.Message{Role: schema.Assistant, Content: "stream"}
	r, w := schema.Pipe[*schema.Message](1)
	go func() { w.Send(msg, nil); w.Close() }()
	return r, nil
}

func (m *mockExploreModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &mockExploreModel{tools: tools}, nil
}

func TestRunBeat_WithFog(t *testing.T) {
	dir, _ := setupTestWorld(t)
	rpgDir := filepath.Join(dir, "rpg")

	// Pre-seed disclosure: hero visible, loc-cavern hidden
	fogStore := fog.NewStore(rpgDir)
	initState := fog.DisclosureState{
		Entities: map[worldmodel.EntityID]fog.EntityDisclosure{
			"hero-arin": {Level: fog.Explored},
		},
	}
	if err := fogStore.Save("world-test-01", initState); err != nil {
		t.Fatalf("seed disclosure: %v", err)
	}

	sess, err := New(Config{
		WorkspacePath: dir,
		RPGDataDir:    rpgDir,
		ChatModel:     &mockExploreModel{},
		MaxStep:       5,
		FogEnabled:    true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID:   "world-test-01",
		UserInput: "I explore the cavern entrance.",
	})
	if err != nil {
		t.Fatalf("RunBeat: %v", err)
	}

	if output.Narrative == "" {
		t.Error("expected narrative")
	}

	// Verify disclosure was persisted with loc-cavern now explored
	updated, err := fogStore.Load("world-test-01")
	if err != nil {
		t.Fatalf("load disclosure: %v", err)
	}
	level := updated.GetEntityLevel("loc-cavern")
	if level != fog.Explored {
		t.Errorf("expected loc-cavern to be explored, got %s", level)
	}
}
