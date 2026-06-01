package runtime

import (
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestEvaluatePreconditionMemoryExistsByID(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{{
			ID:          "mem_alice_evidence",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
			Scope:       model.MemoryScopeFactual,
			Kind:        model.MemoryKindObservation,
			Content:     "Alice saw the broken seal.",
			TruthStatus: model.TruthStatusTrue,
		}},
	}
	condition := model.Condition{
		Kind:     model.ConditionKindMemory,
		Path:     "mem_alice_evidence",
		Operator: model.ConditionOperatorExists,
	}

	ok, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err != nil {
		t.Fatalf("evaluatePrecondition returned error: %v", err)
	}
	if !ok {
		t.Fatal("evaluatePrecondition returned false, want true (memory exists by ID)")
	}
}

func TestEvaluatePreconditionMemoryExistsByOwner(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{{
			ID:          "mem_world_rumor",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Scope:       model.MemoryScopeRumor,
			Kind:        model.MemoryKindRumor,
			Content:     "A monster prowls.",
			TruthStatus: model.TruthStatusUnknown,
		}, {
			ID:          "mem_alice_thought",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "Alice thinks Bob is guilty.",
			TruthStatus: model.TruthStatusUnknown,
		}},
	}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"world owner shorthand", "owner.world", true},
		{"world owner with empty id", "owner.world.", true},
		{"character owner match", "owner.character.char_alice", true},
		{"character owner mismatch", "owner.character.char_bob", false},
		{"unknown owner kind", "owner.faction.house_x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := model.Condition{
				Kind:     model.ConditionKindMemory,
				Path:     tc.path,
				Operator: model.ConditionOperatorExists,
			}
			ok, err := evaluatePrecondition(world, model.WorldEvent{}, c)
			if err != nil {
				t.Fatalf("evaluatePrecondition returned error: %v", err)
			}
			if ok != tc.want {
				t.Fatalf("path=%q got %v, want %v", tc.path, ok, tc.want)
			}
		})
	}
}

func TestEvaluatePreconditionMemoryNotExists(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "test_world", Name: "Test"}
	condition := model.Condition{
		Kind:     model.ConditionKindMemory,
		Path:     "mem_missing",
		Operator: model.ConditionOperatorNotExists,
	}

	ok, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err != nil {
		t.Fatalf("evaluatePrecondition returned error: %v", err)
	}
	if !ok {
		t.Fatal("evaluatePrecondition returned false, want true (memory does not exist)")
	}
}

func TestEvaluatePreconditionMemoryEqualityIsUnsupported(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{{
			ID:      "mem_x",
			Owner:   model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Content: "stuff",
		}},
	}
	condition := model.Condition{
		Kind:     model.ConditionKindMemory,
		Path:     "mem_x",
		Operator: model.ConditionOperatorEqual,
		Value:    model.Value{Kind: model.ValueKindString, Raw: "stuff"},
	}

	_, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err == nil {
		t.Fatal("evaluatePrecondition returned nil, want unsupported-operator error")
	}
	if !strings.Contains(err.Error(), "unsupported memory precondition operator") {
		t.Fatalf("evaluatePrecondition error = %q, want unsupported-memory-operator", err.Error())
	}
}

func TestEvaluatePreconditionStatExists(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentStats: model.NewStatsComponent(map[string]model.Value{
						"hp": {Kind: model.ValueKindNumber, Raw: float64(100)},
					}),
				},
			},
		},
	}

	cases := []struct {
		name string
		key  string
		op   string
		want bool
	}{
		{"existing stat exists", "char_alice.hp", model.ConditionOperatorExists, true},
		{"missing stat exists", "char_alice.mana", model.ConditionOperatorExists, false},
		{"missing stat not_exists", "char_alice.mana", model.ConditionOperatorNotExists, true},
		{"existing stat not_exists", "char_alice.hp", model.ConditionOperatorNotExists, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := model.Condition{
				Kind:     model.ConditionKindStat,
				Path:     tc.key,
				Operator: tc.op,
			}
			ok, err := evaluatePrecondition(world, model.WorldEvent{}, c)
			if err != nil {
				t.Fatalf("evaluatePrecondition returned error: %v", err)
			}
			if ok != tc.want {
				t.Fatalf("path=%q op=%q got %v, want %v", tc.key, tc.op, ok, tc.want)
			}
		})
	}
}

func TestEvaluatePreconditionStatEqual(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentStats: model.NewStatsComponent(map[string]model.Value{
						"hp": {Kind: model.ValueKindNumber, Raw: float64(100)},
					}),
				},
			},
		},
	}

	cases := []struct {
		name  string
		op    string
		value float64
		want  bool
	}{
		{"equal match", model.ConditionOperatorEqual, 100, true},
		{"equal mismatch", model.ConditionOperatorEqual, 99, false},
		{"not_equal match", model.ConditionOperatorNotEqual, 99, true},
		{"not_equal mismatch", model.ConditionOperatorNotEqual, 100, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := model.Condition{
				Kind:     model.ConditionKindStat,
				Path:     "char_alice.hp",
				Operator: tc.op,
				Value:    model.Value{Kind: model.ValueKindNumber, Raw: tc.value},
			}
			ok, err := evaluatePrecondition(world, model.WorldEvent{}, c)
			if err != nil {
				t.Fatalf("evaluatePrecondition returned error: %v", err)
			}
			if ok != tc.want {
				t.Fatalf("op=%q value=%v got %v, want %v", tc.op, tc.value, ok, tc.want)
			}
		})
	}
}

func TestEvaluatePreconditionStatActorShorthand(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentStats: model.NewStatsComponent(map[string]model.Value{
						"hp": {Kind: model.ValueKindNumber, Raw: float64(42)},
					}),
				},
			},
		},
	}
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"char_alice"},
	}
	condition := model.Condition{
		Kind:     model.ConditionKindStat,
		Path:     "actor.hp",
		Operator: model.ConditionOperatorEqual,
		Value:    model.Value{Kind: model.ValueKindNumber, Raw: float64(42)},
	}

	ok, err := evaluatePrecondition(world, event, condition)
	if err != nil {
		t.Fatalf("evaluatePrecondition returned error: %v", err)
	}
	if !ok {
		t.Fatal("evaluatePrecondition returned false, want true (actor.hp == 42)")
	}
}

func TestEvaluatePreconditionStatMissingEntity(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "test_world", Name: "Test"}
	condition := model.Condition{
		Kind:     model.ConditionKindStat,
		Path:     "char_ghost.hp",
		Operator: model.ConditionOperatorExists,
	}

	ok, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err != nil {
		t.Fatalf("evaluatePrecondition returned error: %v, want nil (missing entity is treated as missing stat)", err)
	}
	if ok {
		t.Fatal("evaluatePrecondition returned true, want false (entity missing)")
	}
}

func TestEvaluatePreconditionStatMissingActor(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "test_world", Name: "Test"}
	condition := model.Condition{
		Kind:     model.ConditionKindStat,
		Path:     "actor.hp",
		Operator: model.ConditionOperatorExists,
	}

	_, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err == nil {
		t.Fatal("evaluatePrecondition returned nil, want error when actor shorthand has no actor")
	}
	if !strings.Contains(err.Error(), "actor") {
		t.Fatalf("evaluatePrecondition error = %q, want actor-related", err.Error())
	}
}

func TestEvaluatePreconditionStatMissingStatsComponent(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
		},
	}
	condition := model.Condition{
		Kind:     model.ConditionKindStat,
		Path:     "char_alice.hp",
		Operator: model.ConditionOperatorNotExists,
	}

	ok, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err != nil {
		t.Fatalf("evaluatePrecondition returned error: %v", err)
	}
	if !ok {
		t.Fatal("evaluatePrecondition returned false, want true (no stats component = stat does not exist)")
	}
}

func TestEvaluatePreconditionStatBadPath(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "test_world", Name: "Test"}
	condition := model.Condition{
		Kind:     model.ConditionKindStat,
		Path:     "single_segment",
		Operator: model.ConditionOperatorExists,
	}

	_, err := evaluatePrecondition(world, model.WorldEvent{}, condition)
	if err == nil {
		t.Fatal("evaluatePrecondition returned nil, want error for malformed stat path")
	}
}
