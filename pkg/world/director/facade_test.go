package director_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/world"
	"github.com/sizolity/nobody/pkg/world/director"
)

func TestPublicScriptDirectorProposes(t *testing.T) {
	d := director.NewScriptDirector("script_1", []world.WorldEvent{{
		ID:     "event_1",
		Type:   world.EventTypeNote,
		Source: world.EventSourceDirector,
	}})

	got, err := d.Propose(director.Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_1" {
		t.Fatalf("proposal mismatch: %#v", got)
	}
}

func TestPublicDirectorInterfaceAcceptsAllTypes(t *testing.T) {
	var _ director.Director = director.NewScriptDirector("s", nil)
	var _ director.Director = director.NewEventTableDirector("t", nil)
	var _ director.Director = director.NewRandomDirector("r", nil, nil)
}
