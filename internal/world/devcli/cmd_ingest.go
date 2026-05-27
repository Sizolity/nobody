package devcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sizolity/nobody/internal/world/ingest"
	"github.com/sizolity/nobody/internal/world/store"
)

func runIngestSource(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("ingest-source", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	file := fs.String("file", "", "source file path (txt or md)")
	draftFile := fs.String("draft-file", "", "pre-computed draft JSON file (skips parsing)")
	conflict := fs.String("conflict", "skip", "conflict policy: skip or replace")
	allowDangling := fs.Bool("allow-dangling", false, "allow dangling entity references")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *file == "" {
		fmt.Fprintln(stderr, "ingest-source requires --workspace, --world-id, and --file")
		return 2
	}

	s := store.NewFileStore(*workspace)
	world, err := s.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	doc, err := ingest.LoadSource(*file)
	if err != nil {
		fmt.Fprintf(stderr, "load source: %v\n", err)
		return 1
	}

	var draft ingest.Draft
	if *draftFile != "" {
		data, err := os.ReadFile(*draftFile)
		if err != nil {
			fmt.Fprintf(stderr, "read draft file: %v\n", err)
			return 1
		}
		if err := json.Unmarshal(data, &draft); err != nil {
			fmt.Fprintf(stderr, "parse draft file: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(stderr, "ingest-source requires --draft-file (no built-in parser)")
		return 2
	}

	vr := ingest.ValidateDraft(draft)
	if len(vr.Errors) > 0 {
		fmt.Fprintln(stderr, "draft validation errors:")
		for _, e := range vr.Errors {
			fmt.Fprintf(stderr, "  - %s\n", e)
		}
		return 1
	}
	for _, w := range vr.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}

	opts := ingest.CompileOptions{AllowDanglingRefs: *allowDangling}
	if *conflict == "replace" {
		opts.ConflictPolicy = ingest.ConflictPolicyReplace
	}

	compiled, report, err := ingest.CompileDraft(world, draft, opts)
	if err != nil {
		fmt.Fprintf(stderr, "compile draft: %v\n", err)
		return 1
	}

	if err := s.SaveSnapshot(ctx, compiled); err != nil {
		fmt.Fprintf(stderr, "save world: %v\n", err)
		return 1
	}

	archiveDir := filepath.Join(*workspace, "worlds", *worldID, "sources")
	archive := ingest.NewSourceArchive(archiveDir)
	if err := archive.SaveSource(doc); err != nil {
		fmt.Fprintf(stderr, "warning: save source archive failed: %v\n", err)
	}
	if len(report.Provenance) > 0 {
		if err := archive.SaveProvenance(doc.ID, report.Provenance); err != nil {
			fmt.Fprintf(stderr, "warning: save provenance failed: %v\n", err)
		}
	}

	fmt.Fprintf(stdout, "ingested %s: %d inserted, %d skipped, %d provenance entries\n",
		doc.Filename, report.Inserted, report.Skipped, len(report.Provenance))
	return 0
}
