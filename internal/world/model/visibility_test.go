package model

import "testing"

func TestVisibilityValidateAcceptsAllModes(t *testing.T) {
	modes := []string{
		VisibilityPublic, VisibilityPrivate, VisibilityParticipantsOnly,
		VisibilityLocationOnly, VisibilityFactionOnly, VisibilityGMOnly,
		VisibilityNarratorOnly, VisibilitySecret, VisibilityOwnerOnly,
	}
	for _, mode := range modes {
		v := Visibility{Mode: mode}
		if err := v.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", mode, err)
		}
	}
}

func TestVisibilityValidateRejectsUnknownMode(t *testing.T) {
	v := Visibility{Mode: "invisible"}
	if err := v.Validate(); err == nil {
		t.Fatal("Validate should reject unknown mode")
	}
}

func TestVisibilityValidateAcceptsEmptyMode(t *testing.T) {
	v := Visibility{}
	if err := v.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for zero value", err)
	}
}

func TestVisibilityValidateChecksEntityIDs(t *testing.T) {
	v := Visibility{
		Mode:      VisibilityParticipantsOnly,
		EntityIDs: []EntityID{"valid-id", ""},
	}
	if err := v.Validate(); err == nil {
		t.Fatal("Validate should reject empty entity ID")
	}
}

func TestVisibilityValidateChecksFactionIDs(t *testing.T) {
	v := Visibility{
		Mode:       VisibilityFactionOnly,
		FactionIDs: []EntityID{"valid-id", "bad id!"},
	}
	if err := v.Validate(); err == nil {
		t.Fatal("Validate should reject unsafe faction ID")
	}
}
