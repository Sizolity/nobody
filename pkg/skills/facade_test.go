package skills_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/skills"
)

func TestPublicEmbedderContract(t *testing.T) {
	var embedder skills.Embedder = fixedEmbedder{}
	got, err := embedder.EmbedStrings(context.Background(), []string{"signal"})
	require.NoError(t, err)
	require.Equal(t, [][]float64{{1, 2, 3}}, got)
}

type fixedEmbedder struct{}

func (fixedEmbedder) EmbedStrings(context.Context, []string) ([][]float64, error) {
	return [][]float64{{1, 2, 3}}, nil
}
