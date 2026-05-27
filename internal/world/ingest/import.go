package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
)

// ImportFileResult is the full result of ImportFile.
type ImportFileResult struct {
	SourceDocument SourceDocument
	Chunks         []SourceChunk
	CompileReport  CompileReport
	World          model.World
}

// ValidationError wraps a ValidationReport with errors into an error.
type ValidationError struct {
	Report ValidationReport
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("draft validation failed: %s", strings.Join(e.Report.Errors, "; "))
}

// ImportFile orchestrates the full ingestion pipeline: load source, chunk,
// parse with the provided parser, validate, compile, and return the result.
// It does NOT persist the world; the caller decides whether to save.
func ImportFile(ctx context.Context, world model.World, path string, parser Parser, opts CompileOptions) (ImportFileResult, error) {
	doc, err := LoadSource(path)
	if err != nil {
		return ImportFileResult{}, err
	}

	chunks := Chunk(doc)

	draft, err := parser.Parse(ctx, doc)
	if err != nil {
		return ImportFileResult{}, err
	}

	vr := ValidateDraft(draft)
	if len(vr.Errors) > 0 {
		return ImportFileResult{}, &ValidationError{Report: vr}
	}

	compiled, report, err := CompileDraft(world, draft, opts)
	if err != nil {
		return ImportFileResult{}, err
	}

	return ImportFileResult{
		SourceDocument: doc,
		Chunks:         chunks,
		CompileReport:  report,
		World:          compiled,
	}, nil
}
