package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

func TestPublicFileStoreRoundTripWorld(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	world := narrative.World{ID: "w1", Title: "Signal City"}
	require.NoError(t, st.SaveWorld(ctx, world))

	got, err := st.LoadWorld(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, world, got)
}

func TestPublicStoreInterfaceAcceptsFileStore(t *testing.T) {
	var _ store.Store = store.NewFileStore(t.TempDir())
}
