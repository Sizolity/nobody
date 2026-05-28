package narrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/ingest"
	"github.com/sizolity/nobody/rpg/role"
)

// mockLoreModel is a configurable ToolCallingChatModel for LoreParser tests.
// Behavior matrix:
//   - generateErr     → Generate returns (nil, generateErr)
//   - rawContent set  → Generate returns Content verbatim (used to inject
//     malformed JSON for the lenient-recovery test)
//   - otherwise       → Generate returns Content with payload marshaled
//     to JSON (the happy-path shape DeepSeek emits under json_object mode)
//
// called is set to true the moment WithTools or Generate is entered, so
// TestLoreParser_ParseEmpty can assert that the empty-text short-circuit
// in Parse actually skipped the LLM entirely (not just happened to get an
// empty response). WithTools is kept on the mock for interface compliance
// — LoreParser no longer calls it, but ToolCallingChatModel embeds the
// method so the chatModel field's static type still requires it.
type mockLoreModel struct {
	generateErr error
	payload     loreDraft
	rawContent  string
	called      bool
}

func (m *mockLoreModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	m.called = true
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	var content string
	if m.rawContent != "" {
		content = m.rawContent
	} else {
		b, err := json.Marshal(m.payload)
		if err != nil {
			return nil, err
		}
		content = string(b)
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: content,
	}, nil
}

func (m *mockLoreModel) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

func (m *mockLoreModel) WithTools(_ []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	m.called = true
	return m, nil
}

func TestLoreParser_ParseSuccess(t *testing.T) {
	payload := loreDraft{
		Entities: []ingest.DraftEntity{{
			ID:         "ent_sun_wukong",
			Type:       "character",
			Name:       "孙悟空",
			Aliases:    []string{"美猴王"},
			Confidence: 0.9,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
		Relations: []ingest.DraftRelation{{
			ID:         "rel_wukong_subudhi",
			Type:       "disciple_of",
			SourceID:   "ent_sun_wukong",
			TargetID:   "ent_subudhi",
			Confidence: 0.8,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
		Memories: []ingest.DraftMemory{{
			ID:         "mem_first_meeting",
			OwnerKind:  "world",
			Content:    "悟空初见菩提祖师，被收为弟子。",
			Scope:      "canonical",
			Kind:       "observation",
			Importance: 0.7,
			Confidence: 0.85,
			SourceRefs: []string{"doc-xiyou-01"},
		}},
	}
	parser := NewLoreParser(&mockLoreModel{payload: payload})
	doc := ingest.SourceDocument{ID: "doc-xiyou-01", Text: "悟空拜师菩提祖师..."}

	got, err := parser.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.Canon != nil {
		t.Errorf("Canon should be nil (world-level metadata, not per-beat), got %+v", got.Canon)
	}

	if len(got.Entities) != 1 {
		t.Fatalf("entities: got %d, want 1", len(got.Entities))
	}
	if got.Entities[0].ID != "ent_sun_wukong" || got.Entities[0].Name != "孙悟空" {
		t.Errorf("entity: got %+v", got.Entities[0])
	}
	if got.Entities[0].Confidence != 0.9 {
		t.Errorf("entity confidence: got %v, want 0.9", got.Entities[0].Confidence)
	}

	if len(got.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(got.Relations))
	}
	if got.Relations[0].ID != "rel_wukong_subudhi" {
		t.Errorf("relation: got %+v", got.Relations[0])
	}

	if len(got.Memories) != 1 {
		t.Fatalf("memories: got %d, want 1", len(got.Memories))
	}
	if got.Memories[0].ID != "mem_first_meeting" {
		t.Errorf("memory: got %+v", got.Memories[0])
	}
}

// TestLoreParser_ParseEmpty pins the empty-text short-circuit: when
// doc.Text is whitespace-only, Parse must return ingest.Draft{} immediately
// without contacting the LLM. The mock is rigged so that any LLM contact
// (Generate) both sets called=true and returns an error — so the
// combination (err == nil && !mock.called) is the proof that the
// short-circuit fired. Comment out the short-circuit in lorekeeper.go
// and this test fails on both counts.
func TestLoreParser_ParseEmpty(t *testing.T) {
	mock := &mockLoreModel{
		generateErr: errors.New("llm must not be contacted for empty text"),
	}
	parser := NewLoreParser(mock)
	doc := ingest.SourceDocument{ID: "doc-xiyou-01", Text: "   \n\t  "}

	got, err := parser.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse on whitespace-only text: unexpected error %v (short-circuit should have skipped the LLM)", err)
	}
	if mock.called {
		t.Fatal("Parse contacted the LLM on whitespace-only text; expected short-circuit")
	}
	if got.Canon != nil {
		t.Errorf("Canon should be nil, got %+v", got.Canon)
	}
	if len(got.Entities) != 0 {
		t.Errorf("Entities should be empty, got %+v", got.Entities)
	}
	if len(got.Relations) != 0 {
		t.Errorf("Relations should be empty, got %+v", got.Relations)
	}
	if len(got.Facts) != 0 {
		t.Errorf("Facts should be empty, got %+v", got.Facts)
	}
	if len(got.Threads) != 0 {
		t.Errorf("Threads should be empty, got %+v", got.Threads)
	}
	if len(got.Memories) != 0 {
		t.Errorf("Memories should be empty, got %+v", got.Memories)
	}
}

// TestLoreParser_EmptyContentReturnsError pins the empty-content branch.
// DeepSeek's json_object mode is documented to occasionally return empty
// content; LoreParser must surface that as a soft error rather than
// silently emitting an empty (but valid-looking) draft. Session-layer
// graceful degrade then keeps the beat running.
func TestLoreParser_EmptyContentReturnsError(t *testing.T) {
	parser := NewLoreParser(&mockLoreModel{rawContent: "   \n\t  "})
	_, err := parser.Parse(context.Background(), ingest.SourceDocument{ID: "doc-1", Text: "一些叙事。"})
	if err == nil {
		t.Fatal("expected error from Parse on empty content")
	}
	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("error should mention 'empty content', got: %v", err)
	}
}

func TestLoreParser_GenerateError(t *testing.T) {
	parser := NewLoreParser(&mockLoreModel{generateErr: errors.New("boom")})
	_, err := parser.Parse(context.Background(), ingest.SourceDocument{ID: "doc-1", Text: "一些叙事。"})
	if err == nil {
		t.Fatal("expected error from Parse")
	}
	if !strings.Contains(err.Error(), "lorekeeper generate") {
		t.Errorf("error should wrap %q, got: %v", "lorekeeper generate", err)
	}
}

// TestLoreParser_RecoversFromUnescapedControlChars pins the defensive
// recovery path: even under json_object mode, providers occasionally
// emit content with literal U+0000..U+001F bytes inside string values,
// which RFC 8259 §7 forbids and sonic's strict default rejects. Parse
// must detect the failure, fall back to sonic's lenient config, and
// recover so a beat is not lost.
//
// The embedded Go-level "\n" in badJSON becomes a literal 0x0A byte
// inside the JSON string value — exactly the failure pattern observed
// in production. Strict parsing fails; lenient parsing succeeds.
func TestLoreParser_RecoversFromUnescapedControlChars(t *testing.T) {
	badJSON := "{\"entities\":[{\"id\":\"ent_baigu\",\"type\":\"character\",\"name\":\"白骨夫人\n（白骨精）\",\"confidence\":0.9,\"source_refs\":[\"doc-1\"]}]}"

	parser := NewLoreParser(&mockLoreModel{rawContent: badJSON})
	doc := ingest.SourceDocument{ID: "doc-1", Text: "白骨夫人现身。"}

	got, err := parser.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse should recover from unescaped control chars, got error: %v", err)
	}
	if len(got.Entities) != 1 {
		t.Fatalf("expected 1 entity after recovery, got %d", len(got.Entities))
	}
	if got.Entities[0].ID != "ent_baigu" {
		t.Errorf("entity ID: got %q, want %q", got.Entities[0].ID, "ent_baigu")
	}
	if !strings.Contains(got.Entities[0].Name, "白骨夫人") {
		t.Errorf("entity name should contain 白骨夫人, got %q", got.Entities[0].Name)
	}
	if !strings.Contains(got.Entities[0].Name, "\n") {
		t.Errorf("entity name should preserve embedded newline (re-escaped then decoded), got %q", got.Entities[0].Name)
	}
}

// TestLoreParser_ImplementsRoleLorekeeper is a compile-time assertion that
// *LoreParser satisfies role.Lorekeeper. Body is trivial; the value lives in
// the var-decl above failing to compile if the interface contract drifts.
func TestLoreParser_ImplementsRoleLorekeeper(t *testing.T) {
	var _ role.Lorekeeper = (*LoreParser)(nil)
	t.Log("compile-time check")
}

// TestLoreParser_ImplementsIngestParser is a compile-time assertion that
// *LoreParser satisfies ingest.Parser. Same rationale as the role.Lorekeeper
// assertion: Lorekeeper embeds ingest.Parser, so both must hold.
func TestLoreParser_ImplementsIngestParser(t *testing.T) {
	var _ ingest.Parser = (*LoreParser)(nil)
	t.Log("compile-time check")
}
