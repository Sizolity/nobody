package model

import (
	"reflect"
	"testing"
)

// Stage 6B — every Clone() must produce a fully independent deep copy.
// These tests build a rich value, clone it, mutate every reachable
// pointer / slice / map field of the clone, and assert the original is
// untouched. If a future contributor adds a new slice/map field to one of
// these types and forgets to clone it, exactly one of these tests should
// fail with a "shared reference" mismatch.

func TestVisibilityCloneNilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var v *Visibility
	if got := v.Clone(); got != nil {
		t.Fatalf("Clone() on nil receiver = %#v, want nil", got)
	}
}

func TestVisibilityCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := &Visibility{
		Mode:       VisibilityPrivate,
		EntityIDs:  []EntityID{"char_a", "char_b"},
		FactionIDs: []EntityID{"faction_guild"},
	}
	got := src.Clone()

	if got == src {
		t.Fatal("Clone returned the same pointer")
	}
	got.EntityIDs[0] = "char_z"
	got.FactionIDs[0] = "faction_evil"
	got.Mode = VisibilityPublic

	if src.EntityIDs[0] != "char_a" {
		t.Fatalf("source EntityIDs mutated: %v", src.EntityIDs)
	}
	if src.FactionIDs[0] != "faction_guild" {
		t.Fatalf("source FactionIDs mutated: %v", src.FactionIDs)
	}
	if src.Mode != VisibilityPrivate {
		t.Fatalf("source Mode mutated: %q", src.Mode)
	}
}

func TestValueCloneDeepCopiesRaw(t *testing.T) {
	t.Parallel()

	src := Value{
		Kind: ValueKindObject,
		Raw: map[string]any{
			"nested": map[string]any{"score": float64(1)},
		},
	}
	got := src.Clone()
	got.Raw.(map[string]any)["nested"].(map[string]any)["score"] = float64(99)

	if src.Raw.(map[string]any)["nested"].(map[string]any)["score"] != float64(1) {
		t.Fatalf("source nested score mutated: %#v", src.Raw)
	}
}

func TestWorldTimeCloneCopiesCalendar(t *testing.T) {
	t.Parallel()

	src := WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 100}}
	got := src.Clone()
	got.Calendar["year"] = 999

	if src.Calendar["year"] != 100 {
		t.Fatalf("source Calendar mutated: %v", src.Calendar)
	}
}

func TestMemoryOwnerCloneNilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var o *MemoryOwner
	if got := o.Clone(); got != nil {
		t.Fatalf("Clone on nil receiver = %#v, want nil", got)
	}
}

func TestMemoryDecayCloneNilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var d *MemoryDecay
	if got := d.Clone(); got != nil {
		t.Fatalf("Clone on nil receiver = %#v, want nil", got)
	}
}

func TestForkInfoCloneNilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var f *ForkInfo
	if got := f.Clone(); got != nil {
		t.Fatalf("Clone on nil receiver = %#v, want nil", got)
	}
}

func TestEntityCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := Entity{
		ID:      "char_alice",
		Type:    "character",
		Name:    "Alice",
		Aliases: []string{"Ali", "A"},
		Components: map[string]any{
			ComponentProfile: map[string]any{"title": "knight"},
			ComponentActor:   NewActorComponent(true, []string{"investigate"}),
		},
		State: map[string]Value{
			"mood": {Kind: ValueKindString, Raw: "calm"},
			"loadout": {Kind: ValueKindObject, Raw: map[string]any{
				"weapon": "sword",
			}},
		},
		Tags: []string{"hero", "scholar"},
	}
	got := src.Clone()

	got.Aliases[0] = "changed"
	got.Tags[0] = "changed"
	got.Components["profile"].(map[string]any)["title"] = "wizard"
	got.Components["new_component"] = map[string]any{"x": 1}
	got.State["mood"] = Value{Kind: ValueKindString, Raw: "angry"}
	got.State["loadout"].Raw.(map[string]any)["weapon"] = "axe"

	if src.Aliases[0] != "Ali" {
		t.Fatalf("source Aliases mutated: %v", src.Aliases)
	}
	if src.Tags[0] != "hero" {
		t.Fatalf("source Tags mutated: %v", src.Tags)
	}
	if src.Components["profile"].(map[string]any)["title"] != "knight" {
		t.Fatalf("source profile component mutated: %#v", src.Components["profile"])
	}
	if _, ok := src.Components["new_component"]; ok {
		t.Fatalf("source Components grew an extra key: %v", src.Components)
	}
	if src.State["mood"].Raw != "calm" {
		t.Fatalf("source State[mood] mutated: %#v", src.State["mood"])
	}
	if src.State["loadout"].Raw.(map[string]any)["weapon"] != "sword" {
		t.Fatalf("source nested loadout mutated: %#v", src.State["loadout"])
	}
}

func TestRelationCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := Relation{
		ID:       "rel_1",
		Type:     "ally",
		SourceID: "char_a",
		TargetID: "char_b",
		Visibility: &Visibility{
			Mode:      VisibilityPrivate,
			EntityIDs: []EntityID{"char_a"},
		},
	}
	got := src.Clone()
	if got.Visibility == src.Visibility {
		t.Fatal("Visibility pointer was reused, want fresh allocation")
	}
	got.Visibility.EntityIDs[0] = "char_z"

	if src.Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Visibility.EntityIDs mutated: %v", src.Visibility.EntityIDs)
	}
}

func TestFactCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := Fact{
		ID:        "fact_1",
		SubjectID: "char_a",
		Predicate: "has_inventory",
		Value: Value{Kind: ValueKindObject, Raw: map[string]any{
			"items": []any{"sword", "shield"},
		}},
		Visibility: &Visibility{Mode: VisibilityFactionOnly, FactionIDs: []EntityID{"faction_a"}},
	}
	got := src.Clone()
	got.Value.Raw.(map[string]any)["items"].([]any)[0] = "changed"
	got.Visibility.FactionIDs[0] = "faction_b"
	got.Visibility.Mode = VisibilityPublic

	if src.Value.Raw.(map[string]any)["items"].([]any)[0] != "sword" {
		t.Fatalf("source Value items mutated: %#v", src.Value.Raw)
	}
	if src.Visibility.FactionIDs[0] != "faction_a" {
		t.Fatalf("source Visibility.FactionIDs mutated: %v", src.Visibility.FactionIDs)
	}
	if src.Visibility.Mode != VisibilityFactionOnly {
		t.Fatalf("source Visibility.Mode mutated: %q", src.Visibility.Mode)
	}
}

func TestMemoryRecordCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := MemoryRecord{
		ID:         "mem_1",
		Owner:      MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_a"},
		EventIDs:   []EventID{"evt_1", "evt_2"},
		SubjectIDs: []EntityID{"char_b"},
		Emotion:    map[string]float64{"joy": 0.5, "fear": 0.1},
		Decay:      &MemoryDecay{Mode: MemoryDecayFadeConfidence, HalfLife: "1d"},
		Visibility: &Visibility{Mode: VisibilityPrivate, EntityIDs: []EntityID{"char_a"}},
		CreatedAt:  WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 1}},
		UpdatedAt:  WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 2}},
		LastAccess: WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 3}},
	}
	got := src.Clone()

	if got.Decay == src.Decay {
		t.Fatal("Decay pointer was reused")
	}
	if got.Visibility == src.Visibility {
		t.Fatal("Visibility pointer was reused")
	}

	got.Emotion["joy"] = 0.99
	got.Emotion["new"] = 0.1
	got.EventIDs[0] = "evt_99"
	got.SubjectIDs[0] = "char_z"
	got.Visibility.EntityIDs[0] = "char_z"
	*got.Decay = MemoryDecay{Mode: MemoryDecayArchiveAfter}
	got.CreatedAt.Calendar["year"] = 999
	got.UpdatedAt.Calendar["year"] = 999
	got.LastAccess.Calendar["year"] = 999

	if src.Emotion["joy"] != 0.5 {
		t.Fatalf("source Emotion[joy] mutated: %v", src.Emotion["joy"])
	}
	if _, ok := src.Emotion["new"]; ok {
		t.Fatalf("source Emotion grew an extra key: %v", src.Emotion)
	}
	if src.EventIDs[0] != "evt_1" {
		t.Fatalf("source EventIDs mutated: %v", src.EventIDs)
	}
	if src.SubjectIDs[0] != "char_b" {
		t.Fatalf("source SubjectIDs mutated: %v", src.SubjectIDs)
	}
	if src.Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Visibility mutated: %v", src.Visibility.EntityIDs)
	}
	if src.Decay.Mode != MemoryDecayFadeConfidence {
		t.Fatalf("source Decay mutated: %#v", src.Decay)
	}
	if src.CreatedAt.Calendar["year"] != 1 {
		t.Fatalf("source CreatedAt.Calendar mutated: %v", src.CreatedAt.Calendar)
	}
	if src.UpdatedAt.Calendar["year"] != 2 {
		t.Fatalf("source UpdatedAt.Calendar mutated: %v", src.UpdatedAt.Calendar)
	}
	if src.LastAccess.Calendar["year"] != 3 {
		t.Fatalf("source LastAccess.Calendar mutated: %v", src.LastAccess.Calendar)
	}
}

func TestConditionCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := Condition{
		Kind:     ConditionKindMemory,
		Path:     "mem_1",
		Operator: "exists",
		Value: Value{Kind: ValueKindObject, Raw: map[string]any{
			"threshold": float64(0.5),
		}},
		Owner: &MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_a"},
	}
	got := src.Clone()
	if got.Owner == src.Owner {
		t.Fatal("Owner pointer was reused")
	}
	got.Value.Raw.(map[string]any)["threshold"] = float64(0.99)
	*got.Owner = MemoryOwner{Kind: MemoryOwnerKindFaction, ID: "faction_z"}

	if src.Value.Raw.(map[string]any)["threshold"] != float64(0.5) {
		t.Fatalf("source Condition.Value mutated: %#v", src.Value.Raw)
	}
	if src.Owner.Kind != MemoryOwnerKindCharacter || src.Owner.ID != "char_a" {
		t.Fatalf("source Condition.Owner mutated: %#v", src.Owner)
	}
}

func TestEffectCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := Effect{
		Kind:     EffectUpdateEntityState,
		TargetID: "char_a",
		Payload: map[string]Value{
			"mood": {Kind: ValueKindString, Raw: "calm"},
			"flags": {Kind: ValueKindObject, Raw: map[string]any{
				"alive": true,
			}},
		},
	}
	got := src.Clone()
	got.Payload["mood"] = Value{Kind: ValueKindString, Raw: "angry"}
	got.Payload["flags"].Raw.(map[string]any)["alive"] = false
	got.Payload["new"] = Value{Kind: ValueKindBoolean, Raw: true}

	if src.Payload["mood"].Raw != "calm" {
		t.Fatalf("source Payload[mood] mutated: %#v", src.Payload["mood"])
	}
	if src.Payload["flags"].Raw.(map[string]any)["alive"] != true {
		t.Fatalf("source nested Payload[flags] mutated: %#v", src.Payload["flags"])
	}
	if _, ok := src.Payload["new"]; ok {
		t.Fatalf("source Payload grew an extra key: %v", src.Payload)
	}
}

func TestWorldEventCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := WorldEvent{
		ID:        "evt_1",
		Type:      EventTypeNote,
		Source:    EventSourceRuntime,
		ActorIDs:  []EntityID{"char_a"},
		TargetIDs: []EntityID{"char_b"},
		Preconditions: []Condition{{
			Kind:     ConditionKindMemory,
			Path:     "mem_1",
			Operator: "exists",
			Value:    Value{Kind: ValueKindObject, Raw: map[string]any{"score": float64(1)}},
			Owner:    &MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_a"},
		}},
		Effects: []Effect{{
			Kind:     EffectUpdateEntityState,
			TargetID: "char_a",
			Payload: map[string]Value{
				"mood": {Kind: ValueKindString, Raw: "calm"},
			},
		}},
		Visibility: &Visibility{Mode: VisibilityPrivate, EntityIDs: []EntityID{"char_a"}},
		Causes:     []EventID{"evt_0"},
		Results:    []EventID{"evt_2"},
		OccurredAt: WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 7}},
		Metadata:   map[string]any{"trace_id": "t1", "tags": []any{"a", "b"}},
	}
	got := src.Clone()

	if got.Visibility == src.Visibility {
		t.Fatal("Visibility pointer was reused")
	}
	if got.Preconditions[0].Owner == src.Preconditions[0].Owner {
		t.Fatal("Preconditions[0].Owner pointer was reused")
	}

	got.ActorIDs[0] = "char_z"
	got.TargetIDs[0] = "char_z"
	got.Preconditions[0].Value.Raw.(map[string]any)["score"] = float64(99)
	*got.Preconditions[0].Owner = MemoryOwner{Kind: MemoryOwnerKindFaction, ID: "faction_z"}
	got.Effects[0].Payload["mood"] = Value{Kind: ValueKindString, Raw: "angry"}
	got.Visibility.EntityIDs[0] = "char_z"
	got.Causes[0] = "evt_zzz"
	got.Results[0] = "evt_zzz"
	got.OccurredAt.Calendar["year"] = 999
	got.Metadata["trace_id"] = "tZ"
	got.Metadata["tags"].([]any)[0] = "Z"

	if src.ActorIDs[0] != "char_a" {
		t.Fatalf("source ActorIDs mutated: %v", src.ActorIDs)
	}
	if src.TargetIDs[0] != "char_b" {
		t.Fatalf("source TargetIDs mutated: %v", src.TargetIDs)
	}
	if src.Preconditions[0].Value.Raw.(map[string]any)["score"] != float64(1) {
		t.Fatalf("source Preconditions[0].Value mutated: %#v", src.Preconditions[0].Value.Raw)
	}
	if src.Preconditions[0].Owner.Kind != MemoryOwnerKindCharacter ||
		src.Preconditions[0].Owner.ID != "char_a" {
		t.Fatalf("source Preconditions[0].Owner mutated: %#v", src.Preconditions[0].Owner)
	}
	if src.Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("source Effects[0].Payload[mood] mutated: %#v", src.Effects[0].Payload["mood"])
	}
	if src.Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Visibility.EntityIDs mutated: %v", src.Visibility.EntityIDs)
	}
	if src.Causes[0] != "evt_0" {
		t.Fatalf("source Causes mutated: %v", src.Causes)
	}
	if src.Results[0] != "evt_2" {
		t.Fatalf("source Results mutated: %v", src.Results)
	}
	if src.OccurredAt.Calendar["year"] != 7 {
		t.Fatalf("source OccurredAt.Calendar mutated: %v", src.OccurredAt.Calendar)
	}
	if src.Metadata["trace_id"] != "t1" {
		t.Fatalf("source Metadata[trace_id] mutated: %v", src.Metadata["trace_id"])
	}
	if src.Metadata["tags"].([]any)[0] != "a" {
		t.Fatalf("source Metadata[tags] mutated: %v", src.Metadata["tags"])
	}
}

func TestEventQueueItemCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := EventQueueItem{
		Event:     WorldEvent{ID: "evt_q", Type: EventTypeNote, Source: EventSourceRuntime, ActorIDs: []EntityID{"char_a"}},
		Priority:  3,
		NotBefore: WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 4}},
	}
	got := src.Clone()
	got.Event.ActorIDs[0] = "char_z"
	got.NotBefore.Calendar["year"] = 999

	if src.Event.ActorIDs[0] != "char_a" {
		t.Fatalf("source Event.ActorIDs mutated: %v", src.Event.ActorIDs)
	}
	if src.NotBefore.Calendar["year"] != 4 {
		t.Fatalf("source NotBefore.Calendar mutated: %v", src.NotBefore.Calendar)
	}
}

func TestWorldThreadCloneIsIndependent(t *testing.T) {
	t.Parallel()

	src := WorldThread{
		ID:             "thread_1",
		Kind:           ThreadKindMystery,
		Title:          "Find the killer",
		Status:         ThreadStatusOpen,
		ParticipantIDs: []EntityID{"char_a", "char_b"},
		Visibility:     &Visibility{Mode: VisibilitySecret, EntityIDs: []EntityID{"char_a"}},
		UpdatedBy:      []EventID{"evt_1"},
		Goals: []ThreadGoal{{
			ID:          "goal_1",
			Description: "find the killer",
			DesiredState: []Condition{{
				Kind:     ConditionKindMemory,
				Path:     "mem_1",
				Operator: "exists",
				Value:    Value{Kind: ValueKindObject, Raw: map[string]any{"threshold": float64(0.5)}},
				Owner:    &MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_a"},
			}},
		}},
		Stakes: []ThreadStake{{
			Description: "the witness dies",
			EntityIDs:   []EntityID{"char_witness"},
		}},
		Clues: []ThreadClue{{
			ID:       "clue_1",
			Content:  "muddy boot print",
			KnownBy:  []EntityID{"char_a"},
			PointsTo: []EntityID{"char_suspect"},
		}},
		Branches: []ThreadBranch{{
			TriggerCondition: []Condition{{
				Kind:     ConditionKindState,
				Path:     "actor.state.alive",
				Operator: "==",
				Value:    Value{Kind: ValueKindObject, Raw: map[string]any{"value": true}},
			}},
			ResultHint: "witness flees",
			Weight:     0.7,
		}},
		Deadline: &WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 7}},
	}
	got := src.Clone()

	if got.Visibility == src.Visibility {
		t.Fatal("Visibility pointer was reused")
	}
	if got.Deadline == src.Deadline {
		t.Fatal("Deadline pointer was reused")
	}

	got.ParticipantIDs[0] = "char_z"
	got.UpdatedBy[0] = "evt_z"
	got.Goals[0].DesiredState[0].Value.Raw.(map[string]any)["threshold"] = float64(0.99)
	got.Goals[0].DesiredState[0].Owner.ID = "char_z"
	got.Stakes[0].EntityIDs[0] = "char_z"
	got.Clues[0].KnownBy[0] = "char_z"
	got.Clues[0].PointsTo[0] = "char_z"
	got.Branches[0].TriggerCondition[0].Value.Raw.(map[string]any)["value"] = false
	got.Visibility.EntityIDs[0] = "char_z"
	got.Deadline.Calendar["year"] = 999

	if src.ParticipantIDs[0] != "char_a" {
		t.Fatalf("source ParticipantIDs mutated: %v", src.ParticipantIDs)
	}
	if src.UpdatedBy[0] != "evt_1" {
		t.Fatalf("source UpdatedBy mutated: %v", src.UpdatedBy)
	}
	if src.Goals[0].DesiredState[0].Value.Raw.(map[string]any)["threshold"] != float64(0.5) {
		t.Fatalf("source Goals[0].DesiredState[0].Value mutated: %#v",
			src.Goals[0].DesiredState[0].Value.Raw)
	}
	if src.Goals[0].DesiredState[0].Owner.ID != "char_a" {
		t.Fatalf("source Goals[0].DesiredState[0].Owner mutated: %#v",
			src.Goals[0].DesiredState[0].Owner)
	}
	if src.Stakes[0].EntityIDs[0] != "char_witness" {
		t.Fatalf("source Stakes[0].EntityIDs mutated: %v", src.Stakes[0].EntityIDs)
	}
	if src.Clues[0].KnownBy[0] != "char_a" {
		t.Fatalf("source Clues[0].KnownBy mutated: %v", src.Clues[0].KnownBy)
	}
	if src.Clues[0].PointsTo[0] != "char_suspect" {
		t.Fatalf("source Clues[0].PointsTo mutated: %v", src.Clues[0].PointsTo)
	}
	if src.Branches[0].TriggerCondition[0].Value.Raw.(map[string]any)["value"] != true {
		t.Fatalf("source Branches[0].TriggerCondition mutated: %#v",
			src.Branches[0].TriggerCondition[0].Value.Raw)
	}
	if src.Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Visibility.EntityIDs mutated: %v", src.Visibility.EntityIDs)
	}
	if src.Deadline.Calendar["year"] != 7 {
		t.Fatalf("source Deadline.Calendar mutated: %v", src.Deadline.Calendar)
	}
}

func TestRuleCloneHandlesNilAndMapData(t *testing.T) {
	t.Parallel()

	nilData := Rule{ID: "rule_nil", Kind: "system", Data: nil}
	got := nilData.Clone()
	if got.Data != nil {
		t.Fatalf("nil Data was not preserved: %#v", got.Data)
	}

	src := Rule{
		ID:      "rule_1",
		Kind:    "custom",
		Enabled: true,
		Data: map[string]any{
			"threshold": float64(0.5),
			"actors":    []any{"char_a", "char_b"},
		},
	}
	got = src.Clone()
	got.Data.(map[string]any)["threshold"] = float64(0.99)
	got.Data.(map[string]any)["actors"].([]any)[0] = "char_z"

	if src.Data.(map[string]any)["threshold"] != float64(0.5) {
		t.Fatalf("source Rule.Data[threshold] mutated: %#v", src.Data)
	}
	if src.Data.(map[string]any)["actors"].([]any)[0] != "char_a" {
		t.Fatalf("source Rule.Data[actors] mutated: %#v", src.Data)
	}
}

func TestCanonCloneCopiesAllSlices(t *testing.T) {
	t.Parallel()

	src := Canon{
		Genre:      []string{"mystery"},
		Tone:       []string{"noir"},
		StyleGuide: []string{"short sentences"},
		Premise:    "A king is dead.",
		Laws:       []string{"murder is illegal"},
		Boundaries: []string{"no torture scenes"},
		Secrets:    []EntityID{"char_suspect"},
	}
	got := src.Clone()
	got.Genre[0] = "noir"
	got.Tone[0] = "epic"
	got.StyleGuide[0] = "long sentences"
	got.Laws[0] = "anything goes"
	got.Boundaries[0] = "anything allowed"
	got.Secrets[0] = "char_other"

	want := Canon{
		Genre:      []string{"mystery"},
		Tone:       []string{"noir"},
		StyleGuide: []string{"short sentences"},
		Premise:    "A king is dead.",
		Laws:       []string{"murder is illegal"},
		Boundaries: []string{"no torture scenes"},
		Secrets:    []EntityID{"char_suspect"},
	}
	if !reflect.DeepEqual(src, want) {
		t.Fatalf("source Canon mutated: %#v", src)
	}
}

func TestWorldMetadataCloneCopiesTagsAndFork(t *testing.T) {
	t.Parallel()

	src := WorldMetadata{
		SchemaVersion: "v1",
		Source:        "test",
		Tags:          []string{"debug", "rich"},
		Fork:          &ForkInfo{ParentWorldID: "world_root", ForkSequence: 3},
	}
	got := src.Clone()
	if got.Fork == src.Fork {
		t.Fatal("Fork pointer was reused")
	}
	got.Tags[0] = "changed"
	*got.Fork = ForkInfo{ParentWorldID: "world_z", ForkSequence: 999}

	if src.Tags[0] != "debug" {
		t.Fatalf("source Tags mutated: %v", src.Tags)
	}
	if src.Fork.ParentWorldID != "world_root" || src.Fork.ForkSequence != 3 {
		t.Fatalf("source Fork mutated: %#v", src.Fork)
	}
}

func TestWorldCloneIsFullyIndependent(t *testing.T) {
	t.Parallel()

	src := World{
		ID:          "world_1",
		Name:        "Test World",
		Description: "a test",
		Canon: Canon{
			Genre: []string{"mystery"},
		},
		Clock: WorldClock{
			Current: WorldTime{Kind: WorldTimeCalendar, Calendar: map[string]int{"year": 1}},
		},
		Entities: map[EntityID]Entity{
			"char_a": {
				ID: "char_a", Type: "character", Name: "Alice",
				Aliases: []string{"Ali"},
				Components: map[string]any{
					"profile": map[string]any{"title": "knight"},
				},
				State: map[string]Value{
					"mood": {Kind: ValueKindString, Raw: "calm"},
				},
				Tags: []string{"hero"},
			},
		},
		Relations: []Relation{
			{ID: "rel_1", Type: "ally", SourceID: "char_a", TargetID: "char_b",
				Visibility: &Visibility{Mode: VisibilityPrivate, EntityIDs: []EntityID{"char_a"}}},
		},
		Facts: []Fact{
			{ID: "fact_1", SubjectID: "char_a", Predicate: "is_alive",
				Value: Value{Kind: ValueKindBoolean, Raw: true}},
		},
		Rules: []Rule{
			{ID: "rule_1", Kind: "system", Enabled: true,
				Data: map[string]any{"threshold": float64(0.5)}},
		},
		Threads: []WorldThread{
			{ID: "thread_1", Kind: ThreadKindMystery, Title: "Find the killer",
				Status:         ThreadStatusOpen,
				ParticipantIDs: []EntityID{"char_a"}},
		},
		EventLog: []WorldEvent{
			{ID: "evt_1", Type: EventTypeNote, Source: EventSourceRuntime,
				ActorIDs: []EntityID{"char_a"},
				Metadata: map[string]any{"k": "v"}},
		},
		EventQueue: []EventQueueItem{
			{Event: WorldEvent{ID: "evt_q", Type: EventTypeNote, Source: EventSourceRuntime,
				ActorIDs: []EntityID{"char_a"}}},
		},
		Memory: []MemoryRecord{
			{ID: "mem_1", Owner: MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_a"},
				SubjectIDs: []EntityID{"char_b"},
				Emotion:    map[string]float64{"joy": 0.1}},
		},
		Metadata: WorldMetadata{
			Tags: []string{"debug"},
			Fork: &ForkInfo{ParentWorldID: "world_root"},
		},
	}
	got := src.Clone()

	got.Canon.Genre[0] = "changed"
	got.Clock.Current.Calendar["year"] = 999
	entity := got.Entities["char_a"]
	entity.Tags[0] = "changed"
	entity.State["mood"] = Value{Kind: ValueKindString, Raw: "angry"}
	entity.Components["profile"].(map[string]any)["title"] = "changed"
	got.Entities["char_a"] = entity
	got.Entities["char_new"] = Entity{ID: "char_new", Type: "character", Name: "New"}
	got.Relations[0].Visibility.EntityIDs[0] = "char_z"
	got.Facts[0].Value.Raw = false
	got.Rules[0].Data.(map[string]any)["threshold"] = float64(0.99)
	got.Threads[0].ParticipantIDs[0] = "char_z"
	got.EventLog[0].ActorIDs[0] = "char_z"
	got.EventLog[0].Metadata["k"] = "Z"
	got.EventQueue[0].Event.ActorIDs[0] = "char_z"
	got.Memory[0].SubjectIDs[0] = "char_z"
	got.Memory[0].Emotion["joy"] = 0.99
	got.Metadata.Tags[0] = "changed"
	*got.Metadata.Fork = ForkInfo{ParentWorldID: "world_z"}

	if src.Canon.Genre[0] != "mystery" {
		t.Fatalf("source Canon.Genre mutated: %v", src.Canon.Genre)
	}
	if src.Clock.Current.Calendar["year"] != 1 {
		t.Fatalf("source Clock.Current.Calendar mutated: %v", src.Clock.Current.Calendar)
	}
	if _, ok := src.Entities["char_new"]; ok {
		t.Fatalf("source Entities grew an extra key: %v", src.Entities)
	}
	srcAlice := src.Entities["char_a"]
	if srcAlice.Tags[0] != "hero" {
		t.Fatalf("source Entity Tags mutated: %v", srcAlice.Tags)
	}
	if srcAlice.State["mood"].Raw != "calm" {
		t.Fatalf("source Entity State mutated: %#v", srcAlice.State["mood"])
	}
	if srcAlice.Components["profile"].(map[string]any)["title"] != "knight" {
		t.Fatalf("source Entity Components mutated: %#v", srcAlice.Components)
	}
	if src.Relations[0].Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Relation visibility mutated: %v", src.Relations[0].Visibility.EntityIDs)
	}
	if src.Facts[0].Value.Raw != true {
		t.Fatalf("source Fact.Value mutated: %#v", src.Facts[0].Value)
	}
	if src.Rules[0].Data.(map[string]any)["threshold"] != float64(0.5) {
		t.Fatalf("source Rule.Data mutated: %#v", src.Rules[0].Data)
	}
	if src.Threads[0].ParticipantIDs[0] != "char_a" {
		t.Fatalf("source Thread ParticipantIDs mutated: %v", src.Threads[0].ParticipantIDs)
	}
	if src.EventLog[0].ActorIDs[0] != "char_a" {
		t.Fatalf("source EventLog ActorIDs mutated: %v", src.EventLog[0].ActorIDs)
	}
	if src.EventLog[0].Metadata["k"] != "v" {
		t.Fatalf("source EventLog Metadata mutated: %v", src.EventLog[0].Metadata)
	}
	if src.EventQueue[0].Event.ActorIDs[0] != "char_a" {
		t.Fatalf("source EventQueue mutated: %v", src.EventQueue[0].Event.ActorIDs)
	}
	if src.Memory[0].SubjectIDs[0] != "char_b" {
		t.Fatalf("source Memory SubjectIDs mutated: %v", src.Memory[0].SubjectIDs)
	}
	if src.Memory[0].Emotion["joy"] != 0.1 {
		t.Fatalf("source Memory Emotion mutated: %v", src.Memory[0].Emotion)
	}
	if src.Metadata.Tags[0] != "debug" {
		t.Fatalf("source Metadata.Tags mutated: %v", src.Metadata.Tags)
	}
	if src.Metadata.Fork.ParentWorldID != "world_root" {
		t.Fatalf("source Metadata.Fork mutated: %#v", src.Metadata.Fork)
	}
}

func TestWorldClonePreservesNilSlicesAndMaps(t *testing.T) {
	t.Parallel()

	src := World{ID: "world_empty", Name: "Empty"}
	got := src.Clone()

	if got.Entities != nil {
		t.Fatalf("nil Entities became non-nil: %#v", got.Entities)
	}
	if got.Relations != nil {
		t.Fatalf("nil Relations became non-nil: %#v", got.Relations)
	}
	if got.Facts != nil {
		t.Fatalf("nil Facts became non-nil: %#v", got.Facts)
	}
	if got.Rules != nil {
		t.Fatalf("nil Rules became non-nil: %#v", got.Rules)
	}
	if got.Threads != nil {
		t.Fatalf("nil Threads became non-nil: %#v", got.Threads)
	}
	if got.EventLog != nil {
		t.Fatalf("nil EventLog became non-nil: %#v", got.EventLog)
	}
	if got.EventQueue != nil {
		t.Fatalf("nil EventQueue became non-nil: %#v", got.EventQueue)
	}
	if got.Memory != nil {
		t.Fatalf("nil Memory became non-nil: %#v", got.Memory)
	}
}
