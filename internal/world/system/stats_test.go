package system

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
)

func TestStatsSystemSetStatBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := statsWorld()
	event, err := StatsSystem{}.SetStatEvent(world, "event_set_stat_1", "char_alice", "hp", model.Value{
		Kind: model.ValueKindNumber,
		Raw:  float64(12),
	})
	if err != nil {
		t.Fatalf("SetStatEvent returned error: %v", err)
	}
	if event.ID != "event_set_stat_1" || event.Type != model.EventTypeStatsChanged || event.Source != model.EventSourceRuntime {
		t.Fatalf("event core fields mismatch: %#v", event)
	}
	if len(event.ActorIDs) != 1 || event.ActorIDs[0] != "char_alice" {
		t.Fatalf("actor ids mismatch: %#v", event.ActorIDs)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	stats, ok := got.Entities["char_alice"].StatsComponent()
	if !ok {
		t.Fatalf("stats component missing: %#v", got.Entities["char_alice"].Components)
	}
	if stats.Values["hp"].Raw != float64(12) {
		t.Fatalf("hp = %#v, want 12", stats.Values["hp"])
	}

	original, _ := world.Entities["char_alice"].StatsComponent()
	if original.Values["hp"].Raw != float64(10) {
		t.Fatalf("system mutated input world: %#v", original.Values)
	}
}

func TestStatsSystemAdjustNumberStatBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := statsWorld()
	event, err := StatsSystem{}.AdjustNumberStatEvent(world, "event_adjust_stat_1", "char_alice", "hp", -3)
	if err != nil {
		t.Fatalf("AdjustNumberStatEvent returned error: %v", err)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	stats, _ := got.Entities["char_alice"].StatsComponent()
	if stats.Values["hp"].Raw != float64(7) {
		t.Fatalf("hp = %#v, want 7", stats.Values["hp"])
	}
}

func TestStatsSystemRejectsMissingEntityStatsOrInvalidStat(t *testing.T) {
	t.Parallel()

	world := statsWorld()
	if _, err := (StatsSystem{}).SetStatEvent(world, "event_set_stat_1", "missing", "hp", model.Value{Kind: model.ValueKindNumber, Raw: float64(1)}); err == nil {
		t.Fatal("SetStatEvent returned nil for missing entity")
	}
	entity := world.Entities["char_alice"]
	entity.Components = nil
	world.Entities["char_alice"] = entity
	if _, err := (StatsSystem{}).SetStatEvent(world, "event_set_stat_1", "char_alice", "hp", model.Value{Kind: model.ValueKindNumber, Raw: float64(1)}); err == nil {
		t.Fatal("SetStatEvent returned nil for entity without stats")
	}

	world = statsWorld()
	if _, err := (StatsSystem{}).AdjustNumberStatEvent(world, "event_adjust_stat_1", "char_alice", "missing", 1); err == nil {
		t.Fatal("AdjustNumberStatEvent returned nil for missing stat")
	}
	if _, err := (StatsSystem{}).AdjustNumberStatEvent(world, "event_adjust_stat_1", "char_alice", "mood", 1); err == nil {
		t.Fatal("AdjustNumberStatEvent returned nil for non-number stat")
	}
}

func statsWorld() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentStats: model.NewStatsComponent(map[string]model.Value{
						"hp":   {Kind: model.ValueKindNumber, Raw: float64(10)},
						"mood": {Kind: model.ValueKindString, Raw: "calm"},
					}),
				},
			},
		},
	}
}
