package view_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/world"
	"github.com/sizolity/nobody/pkg/world/view"
)

func TestPublicDebugViewRendersWorld(t *testing.T) {
	w := world.World{ID: "test_world", Name: "Test World"}
	got := view.WorldDebugView{}.Render(w)
	if got.World.ID != "test_world" {
		t.Fatalf("debug view world id mismatch: %#v", got.World)
	}
}

func TestPublicNarrativeViewRendersWorld(t *testing.T) {
	w := world.World{ID: "test_world", Name: "Test World"}
	got := view.NarrativeView{}.Render(w, view.NarrativeContextRequest{})
	if got.World.ID != "test_world" {
		t.Fatalf("narrative view world id mismatch: %#v", got.World)
	}
}
