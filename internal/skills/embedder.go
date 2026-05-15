package skills

import "context"

// Embedder is the minimal embedding contract used by model runtimes and the
// future narrative recall index.
type Embedder interface {
	EmbedStrings(ctx context.Context, texts []string) ([][]float64, error)
}
