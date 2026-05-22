package model

import "testing"

func TestWorldValidateRequiresIDAndName(t *testing.T) {
	world := World{Name: "Test World"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	world = World{ID: "test_world"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without Name")
	}
}

func TestWorldValidateAcceptsMinimalWorld(t *testing.T) {
	world := World{
		ID:   "test_world",
		Name: "Test World",
		Clock: WorldClock{
			Current: WorldTime{Kind: WorldTimeTick, Tick: 1},
		},
	}
	if err := world.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
