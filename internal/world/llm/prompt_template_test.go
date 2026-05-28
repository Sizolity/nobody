package llm

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestPromptTemplateRendersWorldData(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplate(`You direct "{{.Name}}" ({{.WorldID}}). Clock={{.Clock}}. Entities={{len .Entities}}.`)
	if err != nil {
		t.Fatalf("ParsePromptTemplate error: %v", err)
	}

	w := model.World{
		ID:   "w_test",
		Name: "Shadow Kingdom",
		Clock: model.WorldClock{Sequence: 7},
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
			"loc_b":  {ID: "loc_b", Type: "location", Name: "Tavern"},
		},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Shadow Kingdom") {
		t.Errorf("missing world name in %q", got)
	}
	if !strings.Contains(got, "w_test") {
		t.Errorf("missing world ID in %q", got)
	}
	if !strings.Contains(got, "Clock=7") {
		t.Errorf("missing clock in %q", got)
	}
	if !strings.Contains(got, "Entities=2") {
		t.Errorf("missing entity count in %q", got)
	}
}

func TestPromptTemplateCanIterateEntities(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplate(`{{range .Entities}}- {{.Name}} ({{.Type}})` + "\n" + `{{end}}`)
	if err != nil {
		t.Fatalf("ParsePromptTemplate error: %v", err)
	}

	w := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
		},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Alice (character)") {
		t.Errorf("entity not rendered: %q", got)
	}
}

func TestPromptTemplateFStringFormat(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplateWithFormat(`You direct "{Name}" ({WorldID}). Clock={Clock}.`, schema.FString)
	if err != nil {
		t.Fatalf("ParsePromptTemplateWithFormat error: %v", err)
	}

	w := model.World{
		ID:    "w_test",
		Name:  "Shadow Kingdom",
		Clock: model.WorldClock{Sequence: 7},
	}

	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Shadow Kingdom") {
		t.Errorf("missing world name in %q", got)
	}
	if !strings.Contains(got, "w_test") {
		t.Errorf("missing world ID in %q", got)
	}
	if !strings.Contains(got, "Clock=7") {
		t.Errorf("missing clock in %q", got)
	}
}

func TestParsePromptTemplateRejectsInvalid(t *testing.T) {
	t.Parallel()
	_, err := ParsePromptTemplate(`{{.Broken`)
	if err == nil {
		t.Fatal("expected error for broken template")
	}
}

func TestParsePromptTemplateFormatName(t *testing.T) {
	t.Parallel()

	tpl, err := ParsePromptTemplateFormatName(`You direct "{Name}".`, "fstring")
	if err != nil {
		t.Fatalf("ParsePromptTemplateFormatName error: %v", err)
	}

	w := model.World{ID: "w", Name: "TestWorld"}
	got, err := tpl.Render(w)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "TestWorld") {
		t.Errorf("missing world name in %q", got)
	}
}

func TestResolveTemplateFormatRejectsUnknown(t *testing.T) {
	t.Parallel()
	_, err := ResolveTemplateFormat("unknown_format")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
