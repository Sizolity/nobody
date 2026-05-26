package world_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/world"
)

func TestPublicWorldTypesValidate(t *testing.T) {
	w := world.World{ID: "test_world", Name: "Test World"}
	if err := w.Validate(); err != nil {
		t.Fatalf("World.Validate returned error: %v", err)
	}

	e := world.Entity{ID: "char_1", Type: "character", Name: "Alice"}
	if err := e.Validate(); err != nil {
		t.Fatalf("Entity.Validate returned error: %v", err)
	}

	ev := world.WorldEvent{ID: "event_1", Type: world.EventTypeNote, Source: world.EventSourceTest}
	if err := ev.Validate(); err != nil {
		t.Fatalf("WorldEvent.Validate returned error: %v", err)
	}
}

func TestPublicComponentBuildersReturnValidData(t *testing.T) {
	spatial := world.NewSpatialComponent("hall")
	if spatial["location_id"] != "hall" {
		t.Fatalf("NewSpatialComponent: %#v", spatial)
	}

	actor := world.NewActorComponent(true, []string{"explore"})
	if actor["can_act"] != true {
		t.Fatalf("NewActorComponent: %#v", actor)
	}
}

func TestPublicConstantsMatchInternal(t *testing.T) {
	if world.EffectSetFact != "set_fact" {
		t.Fatalf("EffectSetFact = %q", world.EffectSetFact)
	}
	if world.TruthStatusSecret != "secret" {
		t.Fatalf("TruthStatusSecret = %q", world.TruthStatusSecret)
	}
	if world.ThreadKindMystery != "mystery" {
		t.Fatalf("ThreadKindMystery = %q", world.ThreadKindMystery)
	}
}
