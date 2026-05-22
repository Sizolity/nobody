package id_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative/id"
)

func TestValidateAcceptsSafeIDs(t *testing.T) {
	for _, value := range []string{"w1", "world_1", "world-1", "World_2026"} {
		require.NoError(t, id.Validate(value))
		require.True(t, id.IsSafe(value))
	}
}

func TestValidateRejectsUnsafeIDs(t *testing.T) {
	for _, value := range []string{"", "../escape", "world 1", "-world", "_world", "world.json"} {
		require.ErrorContains(t, id.Validate(value), "unsafe id")
		require.False(t, id.IsSafe(value))
	}
}
