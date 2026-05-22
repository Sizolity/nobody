package store

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestFileStoreSaveLoadWorld(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 1},
		},
	}

	if err := st.SaveWorld(ctx, world); err != nil {
		t.Fatalf("SaveWorld returned error: %v", err)
	}
	got, err := st.LoadWorld(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadWorld returned error: %v", err)
	}
	if got.ID != world.ID || got.Name != world.Name {
		t.Fatalf("loaded world mismatch: got %#v want %#v", got, world)
	}
}
