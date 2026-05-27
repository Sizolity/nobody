// Package embedding defines the text-embedding contract for the world framework.
package embedding

import "context"

// Embedder is the minimal embedding contract. Implementations live in the
// product layer (e.g. rpg/) or external packages; the world framework only
// depends on this interface.
type Embedder interface {
	EmbedStrings(ctx context.Context, texts []string) ([][]float64, error)
}
