package store

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestDiffWorldsIdentical(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID: "w", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
		Facts:    []model.Fact{{ID: "f1"}},
		EventLog: []model.WorldEvent{{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}

	d := DiffWorlds(w, w)
	if len(d.Entities.Added) != 0 || len(d.Entities.Removed) != 0 || len(d.Entities.Changed) != 0 {
		t.Fatalf("identical worlds should have no entity diff: %+v", d.Entities)
	}
	if len(d.Facts.Added) != 0 || len(d.Facts.Removed) != 0 {
		t.Fatalf("identical worlds should have no fact diff: %+v", d.Facts)
	}
	if len(d.Events.Added) != 0 || len(d.Events.Removed) != 0 {
		t.Fatalf("identical worlds should have no event diff: %+v", d.Events)
	}
}

func TestDiffWorldsEntityChanges(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "character", Name: "Bob"},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice the Brave"},
			"e3": {ID: "e3", Type: "location", Name: "Market"},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Entities.Added) != 1 || d.Entities.Added[0] != "e3" {
		t.Errorf("added = %v, want [e3]", d.Entities.Added)
	}
	if len(d.Entities.Removed) != 1 || d.Entities.Removed[0] != "e2" {
		t.Errorf("removed = %v, want [e2]", d.Entities.Removed)
	}
	if len(d.Entities.Changed) != 1 || d.Entities.Changed[0] != "e1" {
		t.Errorf("changed = %v, want [e1]", d.Entities.Changed)
	}
}

func TestDiffWorldsThreadStatusChange(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Mystery", Status: model.ThreadStatusOpen},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusResolved},
			{ID: "t3", Kind: model.ThreadKindConflict, Title: "War", Status: model.ThreadStatusActive},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Threads.Added) != 1 || d.Threads.Added[0] != "t3" {
		t.Errorf("threads added = %v, want [t3]", d.Threads.Added)
	}
	if len(d.Threads.Removed) != 1 || d.Threads.Removed[0] != "t2" {
		t.Errorf("threads removed = %v, want [t2]", d.Threads.Removed)
	}
	if len(d.Threads.StatusChanged) != 1 {
		t.Fatalf("status_changed = %d, want 1", len(d.Threads.StatusChanged))
	}
	sc := d.Threads.StatusChanged[0]
	if sc.ID != "t1" || sc.StatusA != model.ThreadStatusActive || sc.StatusB != model.ThreadStatusResolved {
		t.Errorf("status change = %+v", sc)
	}
}

func TestDiffWorldsMemoriesAndEvents(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Memory: []model.MemoryRecord{
			{ID: "m1"}, {ID: "m2"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
	}
	b := model.World{
		ID: "b", Name: "B",
		Memory: []model.MemoryRecord{
			{ID: "m2"}, {ID: "m3"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
			{ID: "ev2", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
	}

	d := DiffWorlds(a, b)
	if len(d.Memories.Added) != 1 || d.Memories.Added[0] != "m3" {
		t.Errorf("memories added = %v, want [m3]", d.Memories.Added)
	}
	if len(d.Memories.Removed) != 1 || d.Memories.Removed[0] != "m1" {
		t.Errorf("memories removed = %v, want [m1]", d.Memories.Removed)
	}
	if len(d.Events.Added) != 1 || d.Events.Added[0] != "ev2" {
		t.Errorf("events added = %v, want [ev2]", d.Events.Added)
	}
	if len(d.Events.Removed) != 0 {
		t.Errorf("events removed = %v, want []", d.Events.Removed)
	}
}

func TestDiffWorldsClockSequence(t *testing.T) {
	t.Parallel()

	a := model.World{ID: "a", Name: "A", Clock: model.WorldClock{Sequence: 5}}
	b := model.World{ID: "b", Name: "B", Clock: model.WorldClock{Sequence: 12}}

	d := DiffWorlds(a, b)
	if d.ClockA != 5 {
		t.Errorf("clock_a = %d, want 5", d.ClockA)
	}
	if d.ClockB != 12 {
		t.Errorf("clock_b = %d, want 12", d.ClockB)
	}
}

func TestDiffWorldsEmptyCollections(t *testing.T) {
	t.Parallel()

	a := model.World{ID: "a", Name: "A"}
	b := model.World{ID: "b", Name: "B"}

	d := DiffWorlds(a, b)
	if d.Entities.Added == nil || d.Entities.Removed == nil || d.Entities.Changed == nil {
		t.Fatal("entity diff slices should be non-nil")
	}
	if d.Facts.Added == nil || d.Facts.Removed == nil {
		t.Fatal("facts diff slices should be non-nil")
	}
	if d.Threads.Added == nil || d.Threads.Removed == nil || d.Threads.StatusChanged == nil {
		t.Fatal("threads diff slices should be non-nil")
	}
}

func TestDiffWorldsRulesAndRelations(t *testing.T) {
	t.Parallel()

	a := model.World{
		ID: "a", Name: "A",
		Rules:     []model.Rule{{ID: "r1"}, {ID: "r2"}},
		Relations: []model.Relation{{ID: "rel1"}},
	}
	b := model.World{
		ID: "b", Name: "B",
		Rules:     []model.Rule{{ID: "r2"}, {ID: "r3"}},
		Relations: []model.Relation{{ID: "rel1"}, {ID: "rel2"}},
	}

	d := DiffWorlds(a, b)
	if len(d.Rules.Added) != 1 || d.Rules.Added[0] != "r3" {
		t.Errorf("rules added = %v, want [r3]", d.Rules.Added)
	}
	if len(d.Rules.Removed) != 1 || d.Rules.Removed[0] != "r1" {
		t.Errorf("rules removed = %v, want [r1]", d.Rules.Removed)
	}
	if len(d.Relations.Added) != 1 || d.Relations.Added[0] != "rel2" {
		t.Errorf("relations added = %v, want [rel2]", d.Relations.Added)
	}
}
