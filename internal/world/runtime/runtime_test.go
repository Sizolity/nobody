package runtime

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestRuntimeRejectsEmptyEvent(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	_, err := rt.ApplyEvent(world, model.WorldEvent{})
	if err == nil {
		t.Fatal("ApplyEvent returned nil for empty event")
	}
}

func TestRuntimeAppliesEventToEventLog(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        "note",
		Source:      "test",
		Description: "test event",
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != event.ID {
		t.Fatalf("EventLog mismatch: %#v", got.EventLog)
	}
}
