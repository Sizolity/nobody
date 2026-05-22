package memstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/memstore"
	"github.com/sizolity/nobody/pkg/narrative/store"
	"github.com/sizolity/nobody/pkg/narrative/storetest"
)

func TestStoreSatisfiesPublicStoreContract(t *testing.T) {
	storetest.RunStoreContract(t, func(t testing.TB) store.Store {
		t.Helper()
		return memstore.New()
	})
}

func TestStoreCopiesValuesOnRead(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()

	graph := narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open", Hooks: []string{"original"}},
		},
	}
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", graph))

	got, err := st.LoadStoryGraph(ctx, "w1")
	require.NoError(t, err)
	got.Nodes[0].Hooks[0] = "mutated"

	again, err := st.LoadStoryGraph(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, "original", again.Nodes[0].Hooks[0])
}
