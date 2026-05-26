package store_test

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/pkg/world"
	"github.com/sizolity/nobody/pkg/world/store"
)

func TestPublicFileStoreRoundTripSnapshot(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	w := world.World{ID: "test_world", Name: "Test World"}
	if err := st.SaveSnapshot(ctx, w); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	got, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if got.ID != "test_world" || got.Name != "Test World" {
		t.Fatalf("snapshot mismatch: %#v", got)
	}
}

func TestPublicStoreInterfaceAcceptsFileStore(t *testing.T) {
	var _ store.Store = store.NewFileStore(t.TempDir())
}
