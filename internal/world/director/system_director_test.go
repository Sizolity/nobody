package director

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestSystemDirectorEmptyWorldReturnsNoProposals(t *testing.T) {
	t.Parallel()

	d := NewSystemDirector("sys_1", SystemDirectorConfig{
		MaxMemoryPerOwner:      10,
		StaleFactThreshold:     0.1,
		EnableConsistencyCheck: true,
	})
	got, err := d.Propose(Context{World: model.World{ID: "world_1", Name: "Empty"}})
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

func TestSystemDirectorMemoryOverflowArchivesLowestImportance(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Memory: []model.MemoryRecord{
			sysTestMemory("mem_a", "char_1", 0.3),
			sysTestMemory("mem_b", "char_1", 0.9),
			sysTestMemory("mem_c", "char_1", 0.1),
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{MaxMemoryPerOwner: 2})
	if d.ID() != "sys_1" {
		t.Fatalf("ID = %q, want sys_1", d.ID())
	}
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	ev := got[0]
	if ev.ID != "sys_archive_mem_c" {
		t.Fatalf("event ID = %q, want sys_archive_mem_c", ev.ID)
	}
	if ev.Type != model.EventTypeRemember {
		t.Fatalf("event Type = %q, want %q", ev.Type, model.EventTypeRemember)
	}
	if ev.Source != model.EventSourceRuntime {
		t.Fatalf("event Source = %q, want %q", ev.Source, model.EventSourceRuntime)
	}
	if len(ev.Effects) != 1 {
		t.Fatalf("effects count = %d, want 1", len(ev.Effects))
	}
	if ev.Effects[0].Kind != model.EffectRemoveMemory || ev.Effects[0].TargetID != "mem_c" {
		t.Fatalf("unexpected effect: %#v", ev.Effects[0])
	}
}

func TestSystemDirectorMaxMemoryPerOwnerZeroMeansNoLimit(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Memory: []model.MemoryRecord{
			sysTestMemory("mem_a", "char_1", 0.1),
			sysTestMemory("mem_b", "char_1", 0.2),
			sysTestMemory("mem_c", "char_1", 0.3),
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{MaxMemoryPerOwner: 0})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0 (no limit): %#v", len(got), got)
	}
}

func TestSystemDirectorStaleFactsBelowThresholdProposed(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Facts: []model.Fact{
			{ID: "fact_stale", SubjectID: "ent_1", Predicate: "old_info", Confidence: 0.05},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{StaleFactThreshold: 0.1})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	ev := got[0]
	if ev.ID != "sys_clean_fact_stale" {
		t.Fatalf("event ID = %q, want sys_clean_fact_stale", ev.ID)
	}
	if ev.Type != model.EventTypeWorldFactChanged {
		t.Fatalf("event Type = %q, want %q", ev.Type, model.EventTypeWorldFactChanged)
	}
	if ev.Source != model.EventSourceRuntime {
		t.Fatalf("event Source = %q, want %q", ev.Source, model.EventSourceRuntime)
	}
	if len(ev.Effects) != 1 || ev.Effects[0].Kind != model.EffectRemoveFact || ev.Effects[0].TargetID != "fact_stale" {
		t.Fatalf("unexpected effect: %#v", ev.Effects)
	}
}

func TestSystemDirectorFactsAtOrAboveThresholdNotProposed(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Facts: []model.Fact{
			{ID: "fact_at", SubjectID: "ent_1", Predicate: "at_threshold", Confidence: 0.1},
			{ID: "fact_above", SubjectID: "ent_1", Predicate: "above", Confidence: 0.5},
			{ID: "fact_zero", SubjectID: "ent_1", Predicate: "zero_conf", Confidence: 0},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{StaleFactThreshold: 0.1})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestSystemDirectorOrphanRelationsProposedWhenEnabled(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"ent_1": {ID: "ent_1", Type: "character", Name: "A"},
		},
		Relations: []model.Relation{
			{ID: "rel_orphan", Type: "knows", SourceID: "ent_1", TargetID: "ent_missing"},
			{ID: "rel_valid", Type: "knows", SourceID: "ent_1", TargetID: "ent_1"},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{EnableConsistencyCheck: true})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	ev := got[0]
	if ev.ID != "sys_clean_rel_orphan" {
		t.Fatalf("event ID = %q, want sys_clean_rel_orphan", ev.ID)
	}
	if ev.Type != model.EventTypeRelationshipChanged {
		t.Fatalf("event Type = %q, want %q", ev.Type, model.EventTypeRelationshipChanged)
	}
	if ev.Source != model.EventSourceRuntime {
		t.Fatalf("event Source = %q, want %q", ev.Source, model.EventSourceRuntime)
	}
	if len(ev.Effects) != 1 || ev.Effects[0].Kind != model.EffectRemoveRelation || ev.Effects[0].TargetID != "rel_orphan" {
		t.Fatalf("unexpected effect: %#v", ev.Effects)
	}
}

func TestSystemDirectorOrphanRelationsIgnoredWhenDisabled(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"ent_1": {ID: "ent_1", Type: "character", Name: "A"},
		},
		Relations: []model.Relation{
			{ID: "rel_orphan", Type: "knows", SourceID: "ent_1", TargetID: "ent_missing"},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{EnableConsistencyCheck: false})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestSystemDirectorCustomPolicyFiresWhenConditionMet(t *testing.T) {
	t.Parallel()

	policy := SystemPolicy{
		ID: "custom_1",
		Condition: func(world model.World) bool {
			return len(world.Entities) > 0
		},
		Propose: func(world model.World) []model.WorldEvent {
			return []model.WorldEvent{{
				ID:     "custom_event_1",
				Type:   model.EventTypeNote,
				Source: model.EventSourceRuntime,
			}}
		},
	}

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"ent_1": {ID: "ent_1", Type: "character", Name: "A"},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{Policies: []SystemPolicy{policy}})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != "custom_event_1" {
		t.Fatalf("event ID = %q, want custom_event_1", got[0].ID)
	}
}

func TestSystemDirectorCustomPolicySkippedWhenConditionFalse(t *testing.T) {
	t.Parallel()

	called := false
	policy := SystemPolicy{
		ID:        "custom_1",
		Condition: func(model.World) bool { return false },
		Propose: func(model.World) []model.WorldEvent {
			called = true
			return []model.WorldEvent{{
				ID:     "custom_event_1",
				Type:   model.EventTypeNote,
				Source: model.EventSourceRuntime,
			}}
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{Policies: []SystemPolicy{policy}})
	got, err := d.Propose(Context{World: model.World{ID: "world_1", Name: "Empty"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if called {
		t.Fatal("custom policy Propose was called despite false condition")
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestSystemDirectorDefaultConfigValues(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test",
		Facts: []model.Fact{
			{ID: "fact_1", SubjectID: "ent_1", Predicate: "stale", Confidence: 0.05},
			{ID: "fact_2", SubjectID: "ent_1", Predicate: "healthy", Confidence: 0.15},
		},
	}

	d := NewSystemDirector("sys_1", SystemDirectorConfig{})
	got, err := d.Propose(Context{World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1 (default StaleFactThreshold=0.1): %#v", len(got), got)
	}
	if got[0].ID != "sys_clean_fact_1" {
		t.Fatalf("event ID = %q, want sys_clean_fact_1", got[0].ID)
	}
}

func sysTestMemory(id model.MemoryID, ownerID string, importance float64) model.MemoryRecord {
	return model.MemoryRecord{
		ID:         id,
		Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: ownerID},
		Scope:      model.MemoryScopeSubjective,
		Kind:       model.MemoryKindObservation,
		Content:    "Test memory " + string(id),
		Importance: importance,
		Confidence: 0.8,
	}
}
