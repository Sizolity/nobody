package system

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
)

func TestActorSystemSetCanActBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := actorWorld()
	event, err := ActorSystem{}.SetCanActEvent(world, "event_actor_1", "char_alice", false)
	if err != nil {
		t.Fatalf("SetCanActEvent returned error: %v", err)
	}
	if event.ID != "event_actor_1" || event.Type != model.EventTypeActorChanged || event.Source != model.EventSourceRuntime {
		t.Fatalf("event core fields mismatch: %#v", event)
	}
	if len(event.ActorIDs) != 1 || event.ActorIDs[0] != "char_alice" {
		t.Fatalf("actor ids mismatch: %#v", event.ActorIDs)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	actor, ok := got.Entities["char_alice"].ActorComponent()
	if !ok {
		t.Fatalf("actor component missing: %#v", got.Entities["char_alice"].Components)
	}
	if actor.CanAct {
		t.Fatalf("CanAct = true, want false")
	}
	if len(actor.Goals) != 1 || actor.Goals[0] != "find the truth" {
		t.Fatalf("goals should be preserved: %#v", actor.Goals)
	}
}

func TestActorSystemSetGoalsBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := actorWorld()
	goals := []string{"escape", "warn Bob"}
	event, err := ActorSystem{}.SetGoalsEvent(world, "event_actor_1", "char_alice", goals)
	if err != nil {
		t.Fatalf("SetGoalsEvent returned error: %v", err)
	}
	goals[0] = "mutated"

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	actor, _ := got.Entities["char_alice"].ActorComponent()
	if !actor.CanAct {
		t.Fatalf("CanAct should be preserved: %#v", actor)
	}
	if len(actor.Goals) != 2 || actor.Goals[0] != "escape" || actor.Goals[1] != "warn Bob" {
		t.Fatalf("goals mismatch: %#v", actor.Goals)
	}

	original, _ := world.Entities["char_alice"].ActorComponent()
	if len(original.Goals) != 1 || original.Goals[0] != "find the truth" {
		t.Fatalf("system mutated input world: %#v", original.Goals)
	}
}

func TestActorSystemCanActReportsCurrentActorState(t *testing.T) {
	t.Parallel()

	canAct, err := ActorSystem{}.CanAct(actorWorld(), "char_alice")
	if err != nil {
		t.Fatalf("CanAct returned error: %v", err)
	}
	if !canAct {
		t.Fatal("CanAct = false, want true")
	}
}

func TestActorSystemRejectsMissingEntityOrActorComponent(t *testing.T) {
	t.Parallel()

	world := actorWorld()
	if _, err := (ActorSystem{}).CanAct(world, "missing"); err == nil {
		t.Fatal("CanAct returned nil for missing entity")
	}
	entity := world.Entities["char_alice"]
	entity.Components = nil
	world.Entities["char_alice"] = entity
	if _, err := (ActorSystem{}).SetCanActEvent(world, "event_actor_1", "char_alice", false); err == nil {
		t.Fatal("SetCanActEvent returned nil for entity without actor component")
	}
}

func actorWorld() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentActor: model.NewActorComponent(true, []string{"find the truth"}),
				},
			},
		},
	}
}
