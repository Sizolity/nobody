package runtime_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/world"
	"github.com/sizolity/nobody/pkg/world/runtime"
)

func TestPublicRuntimeAppliesEvent(t *testing.T) {
	rt := runtime.NewRuntime(runtime.WithoutRules())
	w := world.World{ID: "test_world", Name: "Test World"}
	event := world.WorldEvent{
		ID:     "event_1",
		Type:   world.EventTypeNote,
		Source: world.EventSourceTest,
	}

	got, err := rt.ApplyEvent(w, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != "event_1" {
		t.Fatalf("EventLog mismatch: %#v", got.EventLog)
	}
}

func TestPublicRuntimeStepAdvancesClock(t *testing.T) {
	rt := runtime.NewRuntime(runtime.WithoutRules())
	w := world.World{
		ID:    "test_world",
		Name:  "Test World",
		Clock: world.WorldClock{Sequence: 0},
	}

	got, err := rt.Step(w)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Clock.Sequence != 1 {
		t.Fatalf("Clock.Sequence = %d, want 1", got.World.Clock.Sequence)
	}
}
