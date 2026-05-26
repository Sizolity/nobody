package director

import (
	"context"
	"fmt"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestEinoGeneratorImplementsTextGenerator(t *testing.T) {
	t.Parallel()
	var _ TextGenerator = (*EinoGenerator)(nil)
}

func TestEinoGeneratorImplementsConversationGenerator(t *testing.T) {
	t.Parallel()
	var _ ConversationGenerator = (*EinoGenerator)(nil)
}

func TestEinoGeneratorGenerate(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{response: "hello world"}
	g := NewEinoGenerator(mock)

	got, err := g.Generate(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("content = %q", got)
	}
	if len(mock.lastInput) != 2 {
		t.Fatalf("messages count = %d, want 2", len(mock.lastInput))
	}
	if mock.lastInput[0].Role != schema.System || mock.lastInput[0].Content != "system" {
		t.Errorf("msg[0] = %+v", mock.lastInput[0])
	}
	if mock.lastInput[1].Role != schema.User || mock.lastInput[1].Content != "user" {
		t.Errorf("msg[1] = %+v", mock.lastInput[1])
	}
}

func TestEinoGeneratorGenerateOmitsEmptySystem(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{response: "ok"}
	g := NewEinoGenerator(mock)

	_, err := g.Generate(context.Background(), "", "user only")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(mock.lastInput) != 1 {
		t.Fatalf("messages count = %d, want 1", len(mock.lastInput))
	}
	if mock.lastInput[0].Role != schema.User {
		t.Errorf("expected user role, got %q", mock.lastInput[0].Role)
	}
}

func TestEinoGeneratorRepairSendsFourMessages(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{response: "fixed"}
	g := NewEinoGenerator(mock)

	got, err := g.GenerateRepair(context.Background(), "sys", "orig", "bad", "fix")
	if err != nil {
		t.Fatalf("GenerateRepair error: %v", err)
	}
	if got != "fixed" {
		t.Fatalf("content = %q", got)
	}
	if len(mock.lastInput) != 4 {
		t.Fatalf("messages count = %d, want 4", len(mock.lastInput))
	}
	roles := []schema.RoleType{schema.System, schema.User, schema.Assistant, schema.User}
	contents := []string{"sys", "orig", "bad", "fix"}
	for i, want := range roles {
		if mock.lastInput[i].Role != want {
			t.Errorf("msg[%d].role = %q, want %q", i, mock.lastInput[i].Role, want)
		}
		if mock.lastInput[i].Content != contents[i] {
			t.Errorf("msg[%d].content = %q, want %q", i, mock.lastInput[i].Content, contents[i])
		}
	}
}

func TestEinoGeneratorRepairOmitsEmptySystem(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{response: "ok"}
	g := NewEinoGenerator(mock)

	_, err := g.GenerateRepair(context.Background(), "", "orig", "bad", "fix")
	if err != nil {
		t.Fatalf("GenerateRepair error: %v", err)
	}
	if len(mock.lastInput) != 3 {
		t.Fatalf("messages count = %d, want 3", len(mock.lastInput))
	}
}

func TestEinoGeneratorPropagatesError(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{err: fmt.Errorf("model down")}
	g := NewEinoGenerator(mock)

	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEinoStreamGeneratorAccumulatesChunks(t *testing.T) {
	t.Parallel()

	mock := &mockStreamChatModel{chunks: []string{"hel", "lo ", "world"}}
	g := NewEinoStreamGenerator(mock)

	got, err := g.Generate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("content = %q, want 'hello world'", got)
	}
	if len(mock.lastInput) != 2 {
		t.Fatalf("messages count = %d, want 2", len(mock.lastInput))
	}
}

func TestEinoStreamGeneratorRepairAccumulatesChunks(t *testing.T) {
	t.Parallel()

	mock := &mockStreamChatModel{chunks: []string{`[{"id":`, `"e1","type":"note","source":"director"}]`}}
	g := NewEinoStreamGenerator(mock)

	got, err := g.GenerateRepair(context.Background(), "sys", "orig", "bad", "fix")
	if err != nil {
		t.Fatalf("GenerateRepair error: %v", err)
	}
	if got != `[{"id":"e1","type":"note","source":"director"}]` {
		t.Fatalf("content = %q", got)
	}
	if len(mock.lastInput) != 4 {
		t.Fatalf("messages count = %d, want 4", len(mock.lastInput))
	}
}

func TestEinoStreamGeneratorPropagatesStreamError(t *testing.T) {
	t.Parallel()

	mock := &mockStreamChatModel{streamErr: fmt.Errorf("stream broken")}
	g := NewEinoStreamGenerator(mock)

	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEinoStreamGeneratorPropagatesRecvError(t *testing.T) {
	t.Parallel()

	mock := &mockStreamChatModel{
		chunks:   []string{"partial"},
		chunkErr: fmt.Errorf("network error"),
	}
	g := NewEinoStreamGenerator(mock)

	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error from recv")
	}
}

func TestEinoNonStreamGeneratorIgnoresStreamFlag(t *testing.T) {
	t.Parallel()

	mock := &mockChatModel{response: "non-stream"}
	g := NewEinoGenerator(mock)

	got, err := g.Generate(context.Background(), "", "hi")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if got != "non-stream" {
		t.Fatalf("content = %q", got)
	}
}

func TestNewProviderGeneratorDeepseek(t *testing.T) {
	t.Parallel()

	_, ok := ProviderDefaults["deepseek"]
	if !ok {
		t.Fatal("deepseek not in ProviderDefaults")
	}
}

func TestNewProviderGeneratorRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := NewProviderGenerator(context.Background(), "unknown_llm", "", "key")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewProviderGeneratorRejectsEmptyAPIKey(t *testing.T) {
	t.Parallel()

	_, err := NewProviderGenerator(context.Background(), "deepseek", "", "")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestProviderDefaultsHaveJSONMode(t *testing.T) {
	t.Parallel()

	for name, cfg := range ProviderDefaults {
		if !cfg.JSONMode {
			t.Errorf("provider %q defaults should have JSONMode=true", name)
		}
	}
}

type mockChatModel struct {
	response  string
	err       error
	lastInput []*schema.Message
}

func (m *mockChatModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	m.lastInput = input
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{Role: schema.Assistant, Content: m.response}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream not implemented")
}

type mockStreamChatModel struct {
	chunks    []string
	chunkErr  error
	streamErr error
	lastInput []*schema.Message
}

func (m *mockStreamChatModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	return nil, fmt.Errorf("generate not implemented in stream mock")
}

func (m *mockStreamChatModel) Stream(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	m.lastInput = input
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	reader, writer := schema.Pipe[*schema.Message](len(m.chunks) + 1)
	go func() {
		defer writer.Close()
		for _, chunk := range m.chunks {
			if m.chunkErr != nil {
				writer.Send(nil, m.chunkErr)
				return
			}
			writer.Send(&schema.Message{Role: schema.Assistant, Content: chunk}, nil)
		}
	}()
	return reader, nil
}
