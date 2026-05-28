package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/model"
)

// PromptTemplate wraps an Eino ChatTemplate that renders a system prompt
// using live world state. Variables are injected from PromptTemplateData.
type PromptTemplate struct {
	ct prompt.ChatTemplate
}

// PromptTemplateData is the data available inside a system prompt template.
type PromptTemplateData struct {
	WorldID     string
	Name        string
	Description string
	Clock       int64
	Entities    []EntitySummary
	Facts       []FactSummary
	Relations   []RelationSummary
	Memories    []MemorySummary
	Threads     []ThreadSummary
}

// ParsePromptTemplate parses a template string using Go text/template syntax
// ({{.Var}}). Use ParsePromptTemplateWithFormat for other syntaxes.
func ParsePromptTemplate(text string) (*PromptTemplate, error) {
	return ParsePromptTemplateWithFormat(text, schema.GoTemplate)
}

// ParsePromptTemplateWithFormat parses a template string with the specified
// format type. Supported: schema.GoTemplate ({{.Var}}), schema.FString ({Var}),
// schema.Jinja2 ({{Var}}).
func ParsePromptTemplateWithFormat(text string, ft schema.FormatType) (*PromptTemplate, error) {
	ct := prompt.FromMessages(ft, schema.SystemMessage(text))
	if _, err := ct.Format(context.Background(), templateDataToVars(PromptTemplateData{})); err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}
	return &PromptTemplate{ct: ct}, nil
}

// ParsePromptTemplateFormatName parses a template string with a format
// specified by name. Supported names: "go_template" (default), "fstring",
// "jinja2".
func ParsePromptTemplateFormatName(text, format string) (*PromptTemplate, error) {
	ft, err := ResolveTemplateFormat(format)
	if err != nil {
		return nil, err
	}
	return ParsePromptTemplateWithFormat(text, ft)
}

// ResolveTemplateFormat maps a config string to a schema.FormatType.
func ResolveTemplateFormat(s string) (schema.FormatType, error) {
	switch s {
	case "", "go_template":
		return schema.GoTemplate, nil
	case "fstring":
		return schema.FString, nil
	case "jinja2":
		return schema.Jinja2, nil
	default:
		return 0, fmt.Errorf("unsupported template_format %q (use go_template, fstring, or jinja2)", s)
	}
}

// Render executes the template against the world state and returns the
// resulting system prompt string.
func (pt *PromptTemplate) Render(w model.World) (string, error) {
	data := PromptTemplateData{
		WorldID:     string(w.ID),
		Name:        w.Name,
		Description: w.Description,
		Clock:       w.Clock.Sequence,
		Entities:    EntitySummaries(w.Entities),
		Facts:       FactSummaries(w.Facts),
		Relations:   RelationSummaries(w.Relations),
		Memories:    MemorySummaries(w.Memory),
		Threads:     ThreadSummaries(w.Threads),
	}
	msgs, err := pt.ct.Format(context.Background(), templateDataToVars(data))
	if err != nil {
		return "", fmt.Errorf("render prompt template: %w", err)
	}
	if len(msgs) == 0 {
		return "", nil
	}
	return msgs[0].Content, nil
}

func templateDataToVars(d PromptTemplateData) map[string]any {
	return map[string]any{
		"WorldID":     d.WorldID,
		"Name":        d.Name,
		"Description": d.Description,
		"Clock":       d.Clock,
		"Entities":    d.Entities,
		"Facts":       d.Facts,
		"Relations":   d.Relations,
		"Memories":    d.Memories,
		"Threads":     d.Threads,
	}
}
