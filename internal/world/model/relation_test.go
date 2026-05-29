package model

import "testing"

func TestRelationValidateRequiresCoreFields(t *testing.T) {
	base := Relation{ID: "r1", Type: "knows", SourceID: "e1", TargetID: "e2"}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid relation: %v", err)
	}

	r := base
	r.ID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("should reject empty ID")
	}

	r = base
	r.Type = ""
	if err := r.Validate(); err == nil {
		t.Fatal("should reject empty Type")
	}

	r = base
	r.SourceID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("should reject empty SourceID")
	}

	r = base
	r.TargetID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("should reject empty TargetID")
	}
}

func TestRelationValidateTruthStatus(t *testing.T) {
	r := Relation{ID: "r1", Type: "knows", SourceID: "e1", TargetID: "e2", TruthStatus: TruthStatusDisputed}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid truth status: %v", err)
	}

	r.TruthStatus = "maybe"
	if err := r.Validate(); err == nil {
		t.Fatal("should reject unsupported truth status")
	}
}

func TestRelationValidateConfidenceRange(t *testing.T) {
	r := Relation{ID: "r1", Type: "knows", SourceID: "e1", TargetID: "e2", Confidence: 0.8}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid confidence: %v", err)
	}

	r.Confidence = 1.5
	if err := r.Validate(); err == nil {
		t.Fatal("should reject confidence > 1")
	}

	r.Confidence = -0.1
	if err := r.Validate(); err == nil {
		t.Fatal("should reject confidence < 0")
	}
}

func TestRelationValidateVisibility(t *testing.T) {
	r := Relation{
		ID: "r1", Type: "knows", SourceID: "e1", TargetID: "e2",
		Visibility: &Visibility{Mode: VisibilitySecret},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid visibility: %v", err)
	}

	r.Visibility = &Visibility{Mode: "bogus"}
	if err := r.Validate(); err == nil {
		t.Fatal("should reject invalid visibility mode")
	}
}

func TestRelationAcceptsSourceEvent(t *testing.T) {
	r := Relation{
		ID: "r1", Type: "knows", SourceID: "e1", TargetID: "e2",
		SourceEvent: "evt-1",
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid with source event: %v", err)
	}
}

func TestFactValidateRequiresCoreFields(t *testing.T) {
	base := Fact{ID: "f1", Predicate: "is_locked"}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid fact: %v", err)
	}

	f := base
	f.ID = ""
	if err := f.Validate(); err == nil {
		t.Fatal("should reject empty ID")
	}

	f = base
	f.Predicate = ""
	if err := f.Validate(); err == nil {
		t.Fatal("should reject empty Predicate")
	}
}

func TestFactValidateTruthStatus(t *testing.T) {
	f := Fact{ID: "f1", Predicate: "is_locked", TruthStatus: TruthStatusTrue}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid truth status: %v", err)
	}

	f.TruthStatus = "sorta"
	if err := f.Validate(); err == nil {
		t.Fatal("should reject unsupported truth status")
	}
}

func TestFactValidateConfidenceRange(t *testing.T) {
	f := Fact{ID: "f1", Predicate: "is_locked", Confidence: 0.5}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid confidence: %v", err)
	}

	f.Confidence = 2.0
	if err := f.Validate(); err == nil {
		t.Fatal("should reject confidence > 1")
	}
}

func TestFactValidateVisibility(t *testing.T) {
	f := Fact{
		ID: "f1", Predicate: "is_locked",
		Visibility: &Visibility{Mode: VisibilityGMOnly},
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid visibility: %v", err)
	}

	f.Visibility = &Visibility{Mode: "nope"}
	if err := f.Validate(); err == nil {
		t.Fatal("should reject invalid visibility mode")
	}
}

func TestFactAcceptsSourceEvent(t *testing.T) {
	f := Fact{ID: "f1", Predicate: "is_locked", SourceEvent: "evt-1"}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid with source event: %v", err)
	}
}
