package system_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/world"
	"github.com/sizolity/nobody/pkg/world/system"
)

func TestPublicSpatialSystemBuildsEvent(t *testing.T) {
	w := world.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[world.EntityID]world.Entity{
			"char_1": {
				ID:         "char_1",
				Type:       "character",
				Name:       "Alice",
				Components: map[string]any{world.ComponentSpatial: world.NewSpatialComponent("hall")},
			},
			"tower": {ID: "tower", Type: "location", Name: "Tower"},
		},
	}

	event, err := system.SpatialSystem{}.MoveEvent(w, "event_move", "char_1", "tower")
	if err != nil {
		t.Fatalf("MoveEvent returned error: %v", err)
	}
	if event.Type != world.EventTypeMove {
		t.Fatalf("event type = %q, want move", event.Type)
	}
}
