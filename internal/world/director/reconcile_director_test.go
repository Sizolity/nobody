package director

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestReconcileDirectorProposesReconcileMemoryEvents(t *testing.T) {
	t.Parallel()

	d := NewReconcileDirector("reconcile_1", []ReconcileCase{{
		EventID:          "event_reconcile_1",
		TargetMemoryID:   "memory_1",
		WhenTruthStatus:  model.TruthStatusUnknown,
		TruthStatus:      model.TruthStatusDisputed,
		ConfidenceDelta:  -0.4,
		Summary:          "New evidence disputes this belief.",
		AddMemoryID:      "memory_2",
		AddMemoryContent: "I may have been wrong.",
	}})
	got, err := d.Propose(Context{World: reconcileWorld()})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if d.ID() != "reconcile_1" {
		t.Fatalf("ID = %q, want reconcile_1", d.ID())
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	event := got[0]
	if event.ID != "event_reconcile_1" || event.Type != model.EventTypeRemember || event.Source != model.EventSourceDirector {
		t.Fatalf("unexpected event: %#v", event)
	}
	if len(event.Effects) != 1 {
		t.Fatalf("effects count = %d, want 1: %#v", len(event.Effects), event.Effects)
	}
	effect := event.Effects[0]
	if effect.Kind != model.EffectReconcileMemory || effect.TargetID != "memory_1" {
		t.Fatalf("unexpected effect: %#v", effect)
	}
	if effect.Payload["truth_status"].Raw != model.TruthStatusDisputed {
		t.Fatalf("truth_status payload mismatch: %#v", effect.Payload)
	}
	if effect.Payload["confidence_delta"].Raw != -0.4 {
		t.Fatalf("confidence_delta payload mismatch: %#v", effect.Payload)
	}
	if effect.Payload["summary"].Raw != "New evidence disputes this belief." {
		t.Fatalf("summary payload mismatch: %#v", effect.Payload)
	}
	if effect.Payload["add_memory_id"].Raw != "memory_2" || effect.Payload["add_memory_content"].Raw != "I may have been wrong." {
		t.Fatalf("add memory payload mismatch: %#v", effect.Payload)
	}
}

func TestReconcileDirectorSkipsMissingOrNonMatchingMemories(t *testing.T) {
	t.Parallel()

	d := NewReconcileDirector("reconcile_1", []ReconcileCase{
		{
			EventID:         "event_missing",
			TargetMemoryID:  "missing_memory",
			WhenTruthStatus: model.TruthStatusUnknown,
			TruthStatus:     model.TruthStatusDisputed,
		},
		{
			EventID:         "event_nonmatching",
			TargetMemoryID:  "memory_1",
			WhenTruthStatus: model.TruthStatusTrue,
			TruthStatus:     model.TruthStatusDisputed,
		},
	})
	got, err := d.Propose(Context{World: reconcileWorld()})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if got == nil {
		t.Fatal("events is nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestReconcileDirectorReturnsNonNilEmptySlice(t *testing.T) {
	t.Parallel()

	got, err := NewReconcileDirector("reconcile_1", nil).Propose(Context{World: reconcileWorld()})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if got == nil {
		t.Fatal("events is nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestReconcileDirectorDoesNotAliasReturnedProposals(t *testing.T) {
	t.Parallel()

	d := NewReconcileDirector("reconcile_1", []ReconcileCase{{
		EventID:         "event_reconcile_1",
		TargetMemoryID:  "memory_1",
		WhenTruthStatus: model.TruthStatusUnknown,
		TruthStatus:     model.TruthStatusDisputed,
		Summary:         "Original summary.",
	}})
	first, err := d.Propose(Context{World: reconcileWorld()})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	first[0].Effects[0].Payload["summary"] = model.Value{Kind: model.ValueKindString, Raw: "Changed."}

	second, err := d.Propose(Context{World: reconcileWorld()})
	if err != nil {
		t.Fatalf("second Propose returned error: %v", err)
	}
	if second[0].Effects[0].Payload["summary"].Raw != "Original summary." {
		t.Fatalf("proposal payload was mutated: %#v", second[0].Effects[0].Payload)
	}
}

func reconcileWorld() model.World {
	return model.World{
		ID:   "world_1",
		Name: "World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
		}},
	}
}
