package runtime

import (
	"reflect"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestApplyEventWithoutHooksIsUnchanged(t *testing.T) {
	t.Parallel()

	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "something happened",
		ActorIDs:    []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 0 {
		t.Fatalf("expected no memories without hooks, got %d", len(got.Memory))
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("EventLog length = %d, want 1", len(got.EventLog))
	}
}

func TestAutoExtractMemoryCreatesMemoryPerActor(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoExtractMemory()))
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "The dragon attacked the village",
		ActorIDs:    []model.EntityID{"char_a", "char_b"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 2 {
		t.Fatalf("Memory length = %d, want 2", len(got.Memory))
	}
	for i, actorID := range event.ActorIDs {
		mem := got.Memory[i]
		wantID := model.MemoryID("mem_auto_event_1_" + string(actorID))
		if mem.ID != wantID {
			t.Errorf("Memory[%d].ID = %q, want %q", i, mem.ID, wantID)
		}
		if mem.Owner.Kind != model.MemoryOwnerKindCharacter || mem.Owner.ID != string(actorID) {
			t.Errorf("Memory[%d].Owner = %+v, want character/%s", i, mem.Owner, actorID)
		}
		if mem.Content != event.Description {
			t.Errorf("Memory[%d].Content = %q, want %q", i, mem.Content, event.Description)
		}
		if mem.Scope != model.MemoryScopeFactual {
			t.Errorf("Memory[%d].Scope = %q, want %q", i, mem.Scope, model.MemoryScopeFactual)
		}
		if mem.Kind != model.MemoryKindObservation {
			t.Errorf("Memory[%d].Kind = %q, want %q", i, mem.Kind, model.MemoryKindObservation)
		}
		if mem.TruthStatus != model.TruthStatusTrue {
			t.Errorf("Memory[%d].TruthStatus = %q, want %q", i, mem.TruthStatus, model.TruthStatusTrue)
		}
		if mem.Source != model.MemorySourceDirectExperience {
			t.Errorf("Memory[%d].Source = %q, want %q", i, mem.Source, model.MemorySourceDirectExperience)
		}
		if len(mem.EventIDs) != 1 || mem.EventIDs[0] != event.ID {
			t.Errorf("Memory[%d].EventIDs = %v, want [%s]", i, mem.EventIDs, event.ID)
		}
	}
}

func TestAutoExtractMemorySkipsEmptyDescription(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoExtractMemory()))
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 0 {
		t.Fatalf("expected no memories for empty description, got %d", len(got.Memory))
	}
}

func TestAutoExtractMemorySkipsDuplicateMemoryIDs(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoExtractMemory()))
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Memory: []model.MemoryRecord{{
			ID:          "mem_auto_event_1_char_a",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
			Content:     "pre-existing memory",
			TruthStatus: model.TruthStatusTrue,
		}},
	}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "The dragon attacked",
		ActorIDs:    []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 1 {
		t.Fatalf("Memory length = %d, want 1 (should not duplicate)", len(got.Memory))
	}
	if got.Memory[0].Content != "pre-existing memory" {
		t.Fatalf("existing memory was overwritten: %q", got.Memory[0].Content)
	}
}

func TestAutoUpdateThreadAppendsEventIDToMatchingThreads(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoUpdateThread()))
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Threads: []model.WorldThread{
			{
				ID:             "thread_1",
				Kind:           model.ThreadKindMystery,
				Title:          "Who stole the gem?",
				Status:         model.ThreadStatusOpen,
				ParticipantIDs: []model.EntityID{"char_a", "char_b"},
			},
			{
				ID:             "thread_2",
				Kind:           model.ThreadKindQuest,
				Title:          "Find the ancient scroll",
				Status:         model.ThreadStatusOpen,
				ParticipantIDs: []model.EntityID{"char_c"},
			},
		},
	}
	event := model.WorldEvent{
		ID:        "event_1",
		Type:      model.EventTypeNote,
		Source:    model.EventSourceTest,
		ActorIDs:  []model.EntityID{"char_a"},
		TargetIDs: []model.EntityID{"char_c"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Threads[0].UpdatedBy) != 1 || got.Threads[0].UpdatedBy[0] != "event_1" {
		t.Fatalf("thread_1 UpdatedBy = %v, want [event_1]", got.Threads[0].UpdatedBy)
	}
	if len(got.Threads[1].UpdatedBy) != 1 || got.Threads[1].UpdatedBy[0] != "event_1" {
		t.Fatalf("thread_2 UpdatedBy = %v, want [event_1]", got.Threads[1].UpdatedBy)
	}
}

func TestAutoUpdateThreadIgnoresNonOverlappingThreads(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoUpdateThread()))
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Threads: []model.WorldThread{{
			ID:             "thread_1",
			Kind:           model.ThreadKindMystery,
			Title:          "Unrelated thread",
			Status:         model.ThreadStatusOpen,
			ParticipantIDs: []model.EntityID{"char_x", "char_y"},
		}},
	}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Threads[0].UpdatedBy) != 0 {
		t.Fatalf("thread_1 UpdatedBy = %v, want empty", got.Threads[0].UpdatedBy)
	}
}

func TestMultipleHooksRunInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	hookA := func(world *model.World, event model.WorldEvent) {
		order = append(order, "A")
	}
	hookB := func(world *model.World, event model.WorldEvent) {
		order = append(order, "B")
	}
	hookC := func(world *model.World, event model.WorldEvent) {
		order = append(order, "C")
	}

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(hookA, hookB, hookC))
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	if _, err := rt.ApplyEvent(world, event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(order) != 3 || order[0] != "A" || order[1] != "B" || order[2] != "C" {
		t.Fatalf("hook execution order = %v, want [A B C]", order)
	}
}

func TestAutoExtractMemorySetsCreatedAtAndUpdatedAt(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoExtractMemory()))
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 42, Label: "dawn"},
		},
	}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "The sun rises",
		ActorIDs:    []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 1 {
		t.Fatalf("Memory length = %d, want 1", len(got.Memory))
	}
	mem := got.Memory[0]
	wantTime := model.WorldTime{Kind: model.WorldTimeTick, Tick: 42, Label: "dawn"}
	if !reflect.DeepEqual(mem.CreatedAt, wantTime) {
		t.Errorf("CreatedAt = %+v, want %+v", mem.CreatedAt, wantTime)
	}
	if !reflect.DeepEqual(mem.UpdatedAt, wantTime) {
		t.Errorf("UpdatedAt = %+v, want %+v", mem.UpdatedAt, wantTime)
	}
}

func TestAutoExtractMemoryLeavesLastAccessZero(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(AutoExtractMemory()))
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 10},
		},
	}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "Something happened",
		ActorIDs:    []model.EntityID{"char_a"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	mem := got.Memory[0]
	if !reflect.DeepEqual(mem.LastAccess, model.WorldTime{}) {
		t.Errorf("LastAccess = %+v, want zero value", mem.LastAccess)
	}
}

func TestWithPostApplyHooksOptionWorks(t *testing.T) {
	t.Parallel()

	called := false
	hook := func(world *model.World, event model.WorldEvent) {
		called = true
	}

	rt := NewRuntime(WithoutRules(), WithPostApplyHooks(hook))
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	if _, err := rt.ApplyEvent(world, event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if !called {
		t.Fatal("post-apply hook was not called")
	}
}
