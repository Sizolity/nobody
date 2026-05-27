package ingest

import (
	"context"
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

// Parser extracts structured world data from a source document.
// Implementations may use LLM, rule-based extraction, or any other method.
// The ingest package never provides a concrete implementation.
type Parser interface {
	Parse(ctx context.Context, doc SourceDocument) (Draft, error)
}

// Draft holds the raw extraction output from a Parser before validation
// and compilation into world model types.
type Draft struct {
	Canon     *DraftCanon    `json:"canon,omitempty"`
	Entities  []DraftEntity  `json:"entities,omitempty"`
	Relations []DraftRelation `json:"relations,omitempty"`
	Facts     []DraftFact    `json:"facts,omitempty"`
	Threads   []DraftThread  `json:"threads,omitempty"`
	Memories  []DraftMemory  `json:"memories,omitempty"`
}

type DraftCanon struct {
	Genre      []string `json:"genre,omitempty"`
	Tone       []string `json:"tone,omitempty"`
	Premise    string   `json:"premise,omitempty"`
	Laws       []string `json:"laws,omitempty"`
	Boundaries []string `json:"boundaries,omitempty"`
}

type DraftEntity struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	SourceRefs  []string `json:"source_refs,omitempty"`
}

type DraftRelation struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	SourceID   string   `json:"source_id"`
	TargetID   string   `json:"target_id"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftFact struct {
	ID         string   `json:"id"`
	SubjectID  string   `json:"subject_id"`
	Predicate  string   `json:"predicate"`
	Value      string   `json:"value"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftThread struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary,omitempty"`
	Status     string   `json:"status,omitempty"`
	Priority   float64  `json:"priority,omitempty"`
	Tension    float64  `json:"tension,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type DraftMemory struct {
	ID         string   `json:"id"`
	OwnerKind  string   `json:"owner_kind"`
	OwnerID    string   `json:"owner_id,omitempty"`
	Content    string   `json:"content"`
	Scope      string   `json:"scope,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

// ValidationReport holds errors and warnings from draft validation.
type ValidationReport struct {
	Errors   []string
	Warnings []string
}

// ValidateDraft checks a draft for structural issues without compiling it.
// Returns errors for invalid IDs, missing required fields, and duplicate IDs.
// Returns warnings for dangling references (e.g. facts referencing unknown entities).
func ValidateDraft(draft Draft) ValidationReport {
	var report ValidationReport
	entityIDs := map[string]bool{}

	for i, e := range draft.Entities {
		if e.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(e.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d]: %v", i, err))
		}
		if e.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: name is required", i, e.ID))
		}
		if e.Type == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: type is required", i, e.ID))
		}
		if entityIDs[e.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("entities[%d] %q: duplicate id", i, e.ID))
		}
		entityIDs[e.ID] = true
	}

	relationIDs := map[string]bool{}
	for i, r := range draft.Relations {
		if r.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(r.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d]: %v", i, err))
		}
		if relationIDs[r.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("relations[%d] %q: duplicate id", i, r.ID))
		}
		relationIDs[r.ID] = true
	}

	factIDs := map[string]bool{}
	for i, f := range draft.Facts {
		if f.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(f.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d]: %v", i, err))
		}
		if factIDs[f.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("facts[%d] %q: duplicate id", i, f.ID))
		}
		factIDs[f.ID] = true
		if f.SubjectID != "" && !entityIDs[f.SubjectID] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("facts[%d] %q: subject_id %q not in draft entities", i, f.ID, f.SubjectID))
		}
	}

	threadIDs := map[string]bool{}
	for i, th := range draft.Threads {
		if th.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(th.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d]: %v", i, err))
		}
		if threadIDs[th.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("threads[%d] %q: duplicate id", i, th.ID))
		}
		threadIDs[th.ID] = true
	}

	memoryIDs := map[string]bool{}
	for i, m := range draft.Memories {
		if m.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d]: id is required", i))
			continue
		}
		if err := model.ValidateID(m.ID); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d]: %v", i, err))
		}
		if memoryIDs[m.ID] {
			report.Errors = append(report.Errors, fmt.Sprintf("memories[%d] %q: duplicate id", i, m.ID))
		}
		memoryIDs[m.ID] = true
	}

	return report
}
