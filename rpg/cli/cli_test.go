package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/rule"
)

// mockChatModel handles both invocations the CLI now performs:
//
//  1. ReAct agent — bound to RPG tools; we return plain narrative text so the
//     agent finishes after one model call.
//  2. Narrator.SuggestActions — bound to a single suggest_actions tool; we
//     return a synthetic tool call carrying 2 canned ActionOptions.
//
// The variant is selected via WithTools: any binding whose tool list contains
// "suggest_actions" produces the structured tool-call response.
type mockChatModel struct {
	suggestMode bool
}

func (m *mockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.suggestMode {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "call_suggest",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "suggest_actions",
					Arguments: `{"options":[{"label":"Look around","type":"explore"},{"label":"Wait quietly","type":"rest"}]}`,
				},
			}},
		}, nil
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: "You step into the darkness. The air is cold and damp.",
	}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		w.Send(&schema.Message{Role: schema.Assistant, Content: "stream"}, nil)
		w.Close()
	}()
	return r, nil
}

func (m *mockChatModel) WithTools(toolInfos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	for _, ti := range toolInfos {
		if ti != nil && ti.Name == "suggest_actions" {
			return &mockChatModel{suggestMode: true}, nil
		}
	}
	return &mockChatModel{suggestMode: false}, nil
}

func setupTestWorld(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	combatRule := rule.Rule{
		ID: "rule-01", Category: "combat", Level: 0,
		Content: "Roll d20 for attacks", Source: rule.SourceSystem,
		Enabled: true,
	}
	world := worldmodel.World{
		ID:   "cli-test-world",
		Name: "CLI Test World",
		Canon: worldmodel.Canon{
			Genre: []string{"fantasy"},
			Tone:  []string{"heroic"},
		},
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-01": {
				ID: "hero-01", Type: "character", Name: "TestHero",
				State: map[string]worldmodel.Value{
					"hp": {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
				},
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-01", Kind: worldmodel.ThreadKindQuest,
				Title: "Test Quest", Status: worldmodel.ThreadStatusActive,
			},
		},
		Rules: []worldmodel.Rule{rule.ToModelRule(combatRule)},
		Clock: worldmodel.WorldClock{Sequence: 1},
	}

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(context.Background(), world); err != nil {
		t.Fatalf("save world: %v", err)
	}
	return dir
}

func TestRunBeat_Success(t *testing.T) {
	dir := setupTestWorld(t)
	var stdout, stderr bytes.Buffer

	code := RunBeat(context.Background(), []string{
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--input", "I open the chest.",
	}, &stdout, &stderr, &mockChatModel{})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("expected narrative output on stdout")
	}
	t.Logf("stdout: %s", stdout.String())
	t.Logf("stderr: %s", stderr.String())
}

func TestRunBeat_MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := RunBeat(context.Background(), []string{
		"--workspace", "/tmp",
	}, &stdout, &stderr, &mockChatModel{})

	if code != 2 {
		t.Errorf("expected exit code 2 for missing args, got %d", code)
	}
}

func TestRunBeat_NilModel(t *testing.T) {
	dir := setupTestWorld(t)
	var stdout, stderr bytes.Buffer

	code := RunBeat(context.Background(), []string{
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--input", "test",
	}, &stdout, &stderr, nil)

	if code != 1 {
		t.Errorf("expected exit code 1 for nil model, got %d", code)
	}
}

func TestRunBeat_InvalidWorld(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0o755)
	var stdout, stderr bytes.Buffer

	code := RunBeat(context.Background(), []string{
		"--workspace", dir,
		"--world-id", "nonexistent",
		"--input", "test",
	}, &stdout, &stderr, &mockChatModel{})

	if code != 1 {
		t.Errorf("expected exit code 1 for missing world, got %d", code)
	}
}

func TestRunManageRule_List(t *testing.T) {
	dir := setupTestWorld(t)
	var stdout, stderr bytes.Buffer

	code := RunManageRule(context.Background(), []string{
		"list",
		"--workspace", dir,
		"--world-id", "cli-test-world",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("expected output on stdout")
	}
	t.Logf("stdout: %s", stdout.String())
}

func TestRunManageRule_AddAndRemove(t *testing.T) {
	dir := setupTestWorld(t)
	var stdout, stderr bytes.Buffer

	code := RunManageRule(context.Background(), []string{
		"add",
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--rule-id", "rule-new-01",
		"--content", "New custom rule for testing",
		"--category", "custom",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add: expected code 0, got %d; stderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()

	code = RunManageRule(context.Background(), []string{
		"list",
		"--workspace", dir,
		"--world-id", "cli-test-world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list: expected code 0, got %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("rule-new-01")) {
		t.Error("expected new rule in list output")
	}

	stdout.Reset()
	stderr.Reset()

	code = RunManageRule(context.Background(), []string{
		"remove",
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--rule-id", "rule-new-01",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("remove: expected code 0, got %d; stderr: %s", code, stderr.String())
	}
}

func TestRunManageRule_DisableEnable(t *testing.T) {
	dir := setupTestWorld(t)
	var stdout, stderr bytes.Buffer

	code := RunManageRule(context.Background(), []string{
		"disable",
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--rule-id", "rule-01",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("disable: expected code 0, got %d; stderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()

	code = RunManageRule(context.Background(), []string{
		"enable",
		"--workspace", dir,
		"--world-id", "cli-test-world",
		"--rule-id", "rule-01",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("enable: expected code 0, got %d; stderr: %s", code, stderr.String())
	}
}

func TestRunManageRule_NoSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunManageRule(context.Background(), []string{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected code 2 for no subcommand, got %d", code)
	}
}

func TestRunManageRule_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunManageRule(context.Background(), []string{"unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected code 2 for unknown subcommand, got %d", code)
	}
}
