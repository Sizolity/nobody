package system

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
)

func TestSpatialSystemMoveEntityBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentSpatial: model.NewSpatialComponent("hall"),
				},
			},
			"tower": {ID: "tower", Type: "location", Name: "Tower"},
		},
	}

	event, err := SpatialSystem{}.MoveEvent(world, "event_move_1", "char_alice", "tower")
	if err != nil {
		t.Fatalf("MoveEvent returned error: %v", err)
	}
	if event.ID != "event_move_1" || event.Type != model.EventTypeMove || event.Source != model.EventSourceRuntime {
		t.Fatalf("event core fields mismatch: %#v", event)
	}
	if len(event.ActorIDs) != 1 || event.ActorIDs[0] != "char_alice" || event.LocationID != "tower" {
		t.Fatalf("event participants mismatch: %#v", event)
	}
	if len(event.Effects) != 1 || event.Effects[0].Kind != model.EffectSetEntityComponent {
		t.Fatalf("event effects mismatch: %#v", event.Effects)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	spatial, ok := got.Entities["char_alice"].SpatialComponent()
	if !ok {
		t.Fatalf("spatial component missing: %#v", got.Entities["char_alice"].Components)
	}
	if spatial.LocationID != "tower" {
		t.Fatalf("location id = %q, want tower", spatial.LocationID)
	}

	original, _ := world.Entities["char_alice"].SpatialComponent()
	if original.LocationID != "hall" {
		t.Fatalf("system mutated input world: %#v", world.Entities["char_alice"].Components)
	}
}

func TestSpatialSystemMoveEntityRejectsMissingEntityOrSpatialComponent(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
			"tower":      {ID: "tower", Type: "location", Name: "Tower"},
		},
	}

	if _, err := (SpatialSystem{}).MoveEvent(world, "event_move_1", "missing", "tower"); err == nil {
		t.Fatal("MoveEvent returned nil for missing entity")
	}
	if _, err := (SpatialSystem{}).MoveEvent(world, "event_move_1", "char_alice", "tower"); err == nil {
		t.Fatal("MoveEvent returned nil for entity without spatial component")
	}
	if _, err := (SpatialSystem{}).MoveEvent(world, "event_move_1", "char_alice", "missing_location"); err == nil {
		t.Fatal("MoveEvent returned nil for missing location")
	}
}
