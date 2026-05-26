package bridge

import (
	"testing"

	narr "github.com/sizolity/nobody/internal/narrative"
	"github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/internal/world/model"
)

func TestApplyBeatResultAppendsSceneEvent(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "w", Name: "W"}
	br := BeatResult{
		Draft: narr.Draft{
			ID: "d1", BeatID: "beat_x", Title: "Dawn Breaks",
			Kind: "scene", Text: "The sun rose.",
		},
		MemDelta:   engine.MemoryDelta{Events: []narr.NarrativeEvent{}, Memories: []narr.Memory{}},
		StateDelta: engine.StateDelta{Graph: narr.StoryGraph{}},
	}

	out := ApplyBeatResult(w, br)

	if len(out.EventLog) != 1 {
		t.Fatalf("events = %d, want 1", len(out.EventLog))
	}
	ev := out.EventLog[0]
	if ev.ID != "beat_beat_x" {
		t.Errorf("id = %q", ev.ID)
	}
	if ev.Type != model.EventTypeNote {
		t.Errorf("type = %q", ev.Type)
	}
	if ev.Source != model.EventSourceDirector {
		t.Errorf("source = %q", ev.Source)
	}
	if ev.Intent != "Dawn Breaks" {
		t.Errorf("intent = %q", ev.Intent)
	}
}

func TestApplyBeatResultAppendsNarrativeEvents(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "w", Name: "W"}
	br := BeatResult{
		Draft: narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta: engine.MemoryDelta{
			Events: []narr.NarrativeEvent{
				{ID: "ev_1", BeatID: "b", Type: "discovery", Summary: "Found a key.", ParticipantIDs: []string{"char_a"}},
				{ID: "ev_2", BeatID: "b", Type: "conflict", Summary: "Fight broke out."},
			},
			Memories: []narr.Memory{},
		},
		StateDelta: engine.StateDelta{Graph: narr.StoryGraph{}},
	}

	out := ApplyBeatResult(w, br)

	if len(out.EventLog) != 3 {
		t.Fatalf("events = %d, want 3 (1 scene + 2 narrative)", len(out.EventLog))
	}
	if out.EventLog[1].ID != "ev_1" {
		t.Errorf("event[1] id = %q", out.EventLog[1].ID)
	}
	if out.EventLog[1].Type != model.EventTypeWorldFactChanged {
		t.Errorf("discovery type = %q", out.EventLog[1].Type)
	}
	if len(out.EventLog[1].ActorIDs) != 1 || out.EventLog[1].ActorIDs[0] != "char_a" {
		t.Errorf("actors = %v", out.EventLog[1].ActorIDs)
	}
	if out.EventLog[2].Type != model.EventTypeThreadChanged {
		t.Errorf("conflict type = %q", out.EventLog[2].Type)
	}
}

func TestApplyBeatResultAppendsMemories(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "w", Name: "W"}
	br := BeatResult{
		Draft: narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta: engine.MemoryDelta{
			Events: []narr.NarrativeEvent{},
			Memories: []narr.Memory{
				{ID: "mem_1", Type: "observation", Subject: "world", Text: "The tower crumbled.", Importance: 7},
				{ID: "mem_2", Type: "emotion", Subject: "char_a", Text: "Fear gripped them.", Importance: 4},
			},
		},
		StateDelta: engine.StateDelta{Graph: narr.StoryGraph{}},
	}

	out := ApplyBeatResult(w, br)

	if len(out.Memory) != 2 {
		t.Fatalf("memories = %d, want 2", len(out.Memory))
	}

	m0 := out.Memory[0]
	if m0.ID != "mem_1" {
		t.Errorf("id = %q", m0.ID)
	}
	if m0.Owner.Kind != model.MemoryOwnerKindWorld {
		t.Errorf("owner kind = %q", m0.Owner.Kind)
	}
	if m0.Scope != model.MemoryScopeFactual {
		t.Errorf("scope = %q", m0.Scope)
	}
	if m0.Kind != model.MemoryKindObservation {
		t.Errorf("kind = %q", m0.Kind)
	}
	if m0.Importance != 0.7 {
		t.Errorf("importance = %f, want 0.7", m0.Importance)
	}

	m1 := out.Memory[1]
	if m1.Owner.Kind != model.MemoryOwnerKindCharacter {
		t.Errorf("owner kind = %q", m1.Owner.Kind)
	}
	if m1.Owner.ID != "char_a" {
		t.Errorf("owner id = %q", m1.Owner.ID)
	}
	if m1.Scope != model.MemoryScopeEmotional {
		t.Errorf("scope = %q", m1.Scope)
	}
}

func TestApplyBeatResultReconcileExistingThreadStatus(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID: "w", Name: "W",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Find artifact", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Who is the spy", Status: model.ThreadStatusOpen},
		},
	}
	br := BeatResult{
		Draft: narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta: engine.MemoryDelta{
			Events:   []narr.NarrativeEvent{},
			Memories: []narr.Memory{},
		},
		StateDelta: engine.StateDelta{
			Graph: narr.StoryGraph{
				CurrentNodeID: "t2",
				Nodes: []narr.StoryNode{
					{ID: "t1", Type: "quest", Status: "completed", Goal: "Find artifact"},
					{ID: "t2", Type: "mystery", Status: "active", Goal: "Who is the spy"},
				},
			},
		},
	}

	out := ApplyBeatResult(w, br)

	if len(out.Threads) != 2 {
		t.Fatalf("threads = %d", len(out.Threads))
	}
	if out.Threads[0].Status != model.ThreadStatusResolved {
		t.Errorf("t1 status = %q, want resolved", out.Threads[0].Status)
	}
	if out.Threads[1].Status != model.ThreadStatusActive {
		t.Errorf("t2 status = %q, want active", out.Threads[1].Status)
	}
}

func TestApplyBeatResultAddsNewThreads(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID: "w", Name: "W",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Original", Status: model.ThreadStatusActive},
		},
	}
	br := BeatResult{
		Draft: narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta: engine.MemoryDelta{
			Events:   []narr.NarrativeEvent{},
			Memories: []narr.Memory{},
		},
		StateDelta: engine.StateDelta{
			Graph: narr.StoryGraph{
				CurrentNodeID: "t1",
				Nodes: []narr.StoryNode{
					{ID: "t1", Type: "quest", Status: "active", Goal: "Original"},
					{ID: "t_new", Type: "conflict", Status: "ready", Goal: "New conflict"},
				},
			},
		},
	}

	out := ApplyBeatResult(w, br)

	if len(out.Threads) != 2 {
		t.Fatalf("threads = %d, want 2", len(out.Threads))
	}
	newThread := out.Threads[1]
	if newThread.ID != "t_new" {
		t.Errorf("new thread id = %q", newThread.ID)
	}
	if newThread.Kind != model.ThreadKindConflict {
		t.Errorf("new thread kind = %q", newThread.Kind)
	}
	if newThread.Status != model.ThreadStatusOpen {
		t.Errorf("new thread status = %q", newThread.Status)
	}
	if newThread.Title != "New conflict" {
		t.Errorf("new thread title = %q", newThread.Title)
	}
}

func TestApplyBeatResultAdvancesSequence(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "w", Name: "W"}
	w.Clock.Sequence = 5

	br := BeatResult{
		Draft:      narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta:   engine.MemoryDelta{Events: []narr.NarrativeEvent{}, Memories: []narr.Memory{}},
		StateDelta: engine.StateDelta{Graph: narr.StoryGraph{}},
	}

	out := ApplyBeatResult(w, br)

	if out.Clock.Sequence != 6 {
		t.Errorf("sequence = %d, want 6", out.Clock.Sequence)
	}
	if w.Clock.Sequence != 5 {
		t.Error("original world should not be mutated")
	}
}

func TestApplyBeatResultDoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID: "w", Name: "W",
		EventLog: []model.WorldEvent{{ID: "existing", Type: model.EventTypeNote, Source: model.EventSourceTest}},
		Memory:   []model.MemoryRecord{{ID: "existing_mem", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "old"}},
	}

	br := BeatResult{
		Draft: narr.Draft{ID: "d", BeatID: "b", Title: "T", Kind: "scene", Text: "x"},
		MemDelta: engine.MemoryDelta{
			Events:   []narr.NarrativeEvent{{ID: "new_ev", BeatID: "b", Type: "scene", Summary: "s"}},
			Memories: []narr.Memory{{ID: "new_mem", Type: "observation", Subject: "world", Text: "t", Importance: 1}},
		},
		StateDelta: engine.StateDelta{Graph: narr.StoryGraph{}},
	}

	out := ApplyBeatResult(w, br)

	if len(w.EventLog) != 1 {
		t.Errorf("original event log mutated: %d", len(w.EventLog))
	}
	if len(w.Memory) != 1 {
		t.Errorf("original memory mutated: %d", len(w.Memory))
	}
	if len(out.EventLog) != 3 {
		t.Errorf("output events = %d, want 3", len(out.EventLog))
	}
	if len(out.Memory) != 2 {
		t.Errorf("output memories = %d, want 2", len(out.Memory))
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("this is a longer string", 10); got != "this is..." {
		t.Errorf("truncate long = %q", got)
	}
}
