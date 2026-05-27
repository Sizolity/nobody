package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/rule"
	"github.com/sizolity/nobody/rpg/tools"
)

// mockGM satisfies role.GM with deterministic, LLM-free behavior so the session
// tests can focus on orchestration rather than prompt content. It uses the real
// tools.NewInvokableTools (no progressive disclosure) so all tool calls exercised
// by the ReAct mock — roll, explore_knowledge — are always available.
type mockGM struct{}

func (m *mockGM) Role() string { return "MockGM" }

func (m *mockGM) SystemPrompt(_ []role.Player, opts role.PromptOptions) string {
	return "You are a test GM. World: " + opts.WorldCtx.World.Name
}

func (m *mockGM) Tools(tc *tools.ToolContext) ([]tool.InvokableTool, error) {
	return tools.NewInvokableTools(tc)
}

func (m *mockGM) Judge(_ context.Context, _ role.PlayerAction, _ worldmodel.World) (role.Judgment, error) {
	return role.Judgment{Outcome: "success"}, nil
}

func (m *mockGM) SuggestActions(_ context.Context, _ worldmodel.World, _ []role.Player, _ string) (role.ActionChoices, error) {
	return role.ActionChoices{
		Options: []role.ActionOption{{Label: "test action", Type: role.ActionTypeExplore}},
	}, nil
}

func (m *mockGM) Templates() []role.WorldTemplate { return nil }

// mockChatModel simulates a ToolCallingChatModel:
//
//	1st call: returns a tool call (roll d20)
//	2nd call: returns the final narrative text
type mockChatModel struct {
	callCount int
	tools     []*schema.ToolInfo
}

func (m *mockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
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
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		w.Send(&schema.Message{Role: schema.Assistant, Content: "Streaming not used in test."}, nil)
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

func testPlayers() []role.Player {
	return []role.Player{
		{ID: "p1", CharacterID: "hero-arin", Name: "Tester"},
	}
}

func TestRunBeat_FullPipeline(t *testing.T) {
	dir, _ := setupTestWorld(t)

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		ChatModel:     &mockChatModel{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I push open the ancient door.",
		},
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
	if len(output.Choices.Options) == 0 {
		t.Error("expected at least one suggested action option")
	}

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
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		ChatModel:     &mockChatModel{},
		MaxStep:       5,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I attack the stone golem.",
		},
	})
	if err != nil {
		t.Fatalf("RunBeat: %v", err)
	}

	if output.Narrative == "" {
		t.Error("expected non-empty narrative")
	}
	if !strings.Contains(output.Narrative, "ancient door") {
		t.Errorf("unexpected narrative: %q", output.Narrative)
	}
}

func TestNew_Validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"empty config", Config{}},
		{"missing GM", Config{WorkspacePath: "/tmp/test", ChatModel: &mockChatModel{}}},
		{"missing workspace", Config{GM: &mockGM{}, ChatModel: &mockChatModel{}}},
		{"missing chat model", Config{GM: &mockGM{}, WorkspacePath: "/tmp/test"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.cfg); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
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
	r, w := schema.Pipe[*schema.Message](1)
	go func() { w.Send(&schema.Message{Role: schema.Assistant, Content: "stream"}, nil); w.Close() }()
	return r, nil
}

func (m *mockExploreModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &mockExploreModel{tools: tools}, nil
}

func TestRunBeat_WithFog(t *testing.T) {
	dir, _ := setupTestWorld(t)
	// fog data now colocates with world data under {workspace}/worlds/{worldID}/
	worldsDir := filepath.Join(dir, "worlds")

	// Pre-seed disclosure: hero visible, loc-cavern hidden
	fogStore := fog.NewStore(worldsDir)
	initState := fog.DisclosureState{
		Entities: map[worldmodel.EntityID]fog.EntityDisclosure{
			"hero-arin": {Level: fog.Explored},
		},
	}
	if err := fogStore.Save("world-test-01", initState); err != nil {
		t.Fatalf("seed disclosure: %v", err)
	}

	sess, err := New(Config{
		GM:            &mockGM{},
		Players:       testPlayers(),
		WorkspacePath: dir,
		ChatModel:     &mockExploreModel{},
		MaxStep:       5,
		FogEnabled:    true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	output, err := sess.RunBeat(context.Background(), BeatInput{
		WorldID: "world-test-01",
		Action: role.PlayerAction{
			PlayerID: "p1",
			Content:  "I explore the cavern entrance.",
		},
	})
	if err != nil {
		t.Fatalf("RunBeat: %v", err)
	}

	if output.Narrative == "" {
		t.Error("expected narrative")
	}

	updated, err := fogStore.Load("world-test-01")
	if err != nil {
		t.Fatalf("load disclosure: %v", err)
	}
	level := updated.GetEntityLevel("loc-cavern")
	if level != fog.Explored {
		t.Errorf("expected loc-cavern to be explored, got %s", level)
	}
}
