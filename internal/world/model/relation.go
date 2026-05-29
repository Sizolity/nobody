package model

import "fmt"

type Relation struct {
	ID       RelationID `json:"id"`
	Type     string     `json:"type"`
	SourceID EntityID   `json:"source_id"`
	TargetID EntityID   `json:"target_id"`

	TruthStatus string      `json:"truth_status,omitempty"`
	Confidence  float64     `json:"confidence,omitempty"`
	Visibility  *Visibility `json:"visibility,omitempty"`
	SourceEvent EventID     `json:"source_event,omitempty"`
}

func (r Relation) Validate() error {
	if err := ValidateID(string(r.ID)); err != nil {
		return fmt.Errorf("relation.id: %w", err)
	}
	if r.Type == "" {
		return fmt.Errorf("relation.type is required")
	}
	if r.SourceID == "" {
		return fmt.Errorf("relation.source_id is required")
	}
	if r.TargetID == "" {
		return fmt.Errorf("relation.target_id is required")
	}
	if r.TruthStatus != "" && !isSupportedTruthStatus(r.TruthStatus) {
		return fmt.Errorf("unsupported relation truth status %q", r.TruthStatus)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("relation.confidence must be between 0 and 1")
	}
	if r.Visibility != nil {
		if err := r.Visibility.Validate(); err != nil {
			return fmt.Errorf("relation.%w", err)
		}
	}
	return nil
}

type Fact struct {
	ID        FactID   `json:"id"`
	SubjectID EntityID `json:"subject_id"`
	Predicate string   `json:"predicate"`
	Value     Value    `json:"value"`

	TruthStatus string      `json:"truth_status,omitempty"`
	Confidence  float64     `json:"confidence,omitempty"`
	Visibility  *Visibility `json:"visibility,omitempty"`
	SourceEvent EventID     `json:"source_event,omitempty"`
}

func (f Fact) Validate() error {
	if err := ValidateID(string(f.ID)); err != nil {
		return fmt.Errorf("fact.id: %w", err)
	}
	if f.Predicate == "" {
		return fmt.Errorf("fact.predicate is required")
	}
	if f.TruthStatus != "" && !isSupportedTruthStatus(f.TruthStatus) {
		return fmt.Errorf("unsupported fact truth status %q", f.TruthStatus)
	}
	if f.Confidence < 0 || f.Confidence > 1 {
		return fmt.Errorf("fact.confidence must be between 0 and 1")
	}
	if f.Visibility != nil {
		if err := f.Visibility.Validate(); err != nil {
			return fmt.Errorf("fact.%w", err)
		}
	}
	return nil
}
