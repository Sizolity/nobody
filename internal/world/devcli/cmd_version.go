package devcli

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/runner"
	"github.com/sizolity/nobody/internal/world/store"
)

func runCheckpoint(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("checkpoint", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "checkpoint requires --workspace and --world-id")
		return 2
	}
	r := runner.New(store.NewFileStore(*workspace))
	result, err := r.Checkpoint(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "checkpoint failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "checkpoint saved for world %s at sequence %d\n", result.WorldID, result.Sequence)
	return 0
}

func runRollback(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("rollback", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "rollback requires --workspace, --world-id, and a sequence number as positional arg")
		return 2
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(stderr, "rollback requires exactly one positional arg: the target sequence number")
		return 2
	}
	seq, err := strconv.ParseInt(remaining[0], 10, 64)
	if err != nil {
		fmt.Fprintf(stderr, "invalid sequence number %q: %v\n", remaining[0], err)
		return 2
	}
	r := runner.New(store.NewFileStore(*workspace))
	result, err := r.Rollback(ctx, *worldID, seq)
	if err != nil {
		fmt.Fprintf(stderr, "rollback failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "rolled back world %s to sequence %d\n", result.World.ID, result.RestoredSequence)
	return 0
}

func runListCheckpoints(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("list-checkpoints", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "list-checkpoints requires --workspace and --world-id")
		return 2
	}
	r := runner.New(store.NewFileStore(*workspace))
	seqs, err := r.ListCheckpoints(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "list-checkpoints failed: %v\n", err)
		return 1
	}
	if len(seqs) == 0 {
		fmt.Fprintln(stdout, "no checkpoints")
		return 0
	}
	for _, seq := range seqs {
		fmt.Fprintf(stdout, "checkpoint at sequence %d\n", seq)
	}
	return 0
}

func runFork(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("fork", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "source world id")
	newID := fs.String("new-id", "", "new world id for the fork")
	atSeq := fs.Int64("at-sequence", 0, "fork from checkpoint at this sequence (0 = current state)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *newID == "" {
		fmt.Fprintln(stderr, "fork requires --workspace, --world-id, and --new-id")
		return 2
	}
	r := runner.New(store.NewFileStore(*workspace))
	result, err := r.Fork(ctx, *worldID, *newID, *atSeq)
	if err != nil {
		fmt.Fprintf(stderr, "fork failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "forked world %s from %s at sequence %d\n", result.World.ID, *worldID, result.ForkSequence)
	return 0
}

func runLineage(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("lineage", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	query := fs.String("query", "tree", "query type: ancestors, children, siblings, tree")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "lineage requires --workspace and --world-id")
		return 2
	}

	st := store.NewFileStore(*workspace)

	type lineageOutput struct {
		WorldID string             `json:"world_id"`
		Query   string             `json:"query"`
		Nodes   []store.LineageNode `json:"nodes"`
	}

	var nodes []store.LineageNode
	var err error

	switch *query {
	case "ancestors":
		nodes, err = store.Ancestors(ctx, st, *worldID)
	case "children":
		nodes, err = store.Children(ctx, st, *worldID)
	case "siblings":
		nodes, err = store.Siblings(ctx, st, *worldID)
	case "tree":
		nodes, err = store.LineageTree(ctx, st, *worldID)
	default:
		fmt.Fprintf(stderr, "unknown query type %q (use ancestors, children, siblings, tree)\n", *query)
		return 2
	}

	if err != nil {
		fmt.Fprintf(stderr, "lineage %s failed: %v\n", *query, err)
		return 1
	}

	return writeJSON(stdout, stderr, "lineage output", lineageOutput{
		WorldID: *worldID,
		Query:   *query,
		Nodes:   nodes,
	})
}

func runDiff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("diff", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldA := fs.String("world-a", "", "first world id")
	worldB := fs.String("world-b", "", "second world id")
	format := fs.String("format", "json", "output format: json or text")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldA == "" || *worldB == "" {
		fmt.Fprintln(stderr, "diff requires --workspace, --world-a, and --world-b")
		return 2
	}

	st := store.NewFileStore(*workspace)
	a, err := st.LoadSnapshot(ctx, *worldA)
	if err != nil {
		fmt.Fprintf(stderr, "load world %s: %v\n", *worldA, err)
		return 1
	}
	b, err := st.LoadSnapshot(ctx, *worldB)
	if err != nil {
		fmt.Fprintf(stderr, "load world %s: %v\n", *worldB, err)
		return 1
	}

	delta := store.DiffWorlds(a, b)

	if *format == "text" {
		fmt.Fprint(stdout, store.FormatDiff(delta))
		return 0
	}
	return writeJSON(stdout, stderr, "diff output", delta)
}

func runMerge(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("merge", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	baseID := fs.String("base", "", "common ancestor world id")
	sourceID := fs.String("source", "", "source branch world id (changes to merge from)")
	targetID := fs.String("target", "", "target branch world id (merge into)")
	apply := fs.Bool("apply", false, "save the merged world back to the target")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *baseID == "" || *sourceID == "" || *targetID == "" {
		fmt.Fprintln(stderr, "merge requires --workspace, --base, --source, and --target")
		return 2
	}

	st := store.NewFileStore(*workspace)
	base, err := st.LoadSnapshot(ctx, *baseID)
	if err != nil {
		fmt.Fprintf(stderr, "load base %s: %v\n", *baseID, err)
		return 1
	}
	source, err := st.LoadSnapshot(ctx, *sourceID)
	if err != nil {
		fmt.Fprintf(stderr, "load source %s: %v\n", *sourceID, err)
		return 1
	}
	target, err := st.LoadSnapshot(ctx, *targetID)
	if err != nil {
		fmt.Fprintf(stderr, "load target %s: %v\n", *targetID, err)
		return 1
	}

	merged, report := store.MergeWorlds(base, source, target)

	if report.HasConflicts() {
		fmt.Fprintf(stderr, "merge: %d conflict(s)\n", len(report.Conflicts))
		for _, c := range report.Conflicts {
			fmt.Fprintf(stderr, "  conflict [%s] %s: %s\n", c.Kind, c.ID, c.Desc)
		}
	}

	if *apply {
		if report.HasConflicts() {
			fmt.Fprintln(stderr, "merge: refusing to apply with unresolved conflicts")
			return writeJSON(stdout, stderr, "merge report", report)
		}
		if err := st.SaveSnapshot(ctx, merged); err != nil {
			fmt.Fprintf(stderr, "save merged world: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "merge: applied to %s (sequence %d)\n", merged.ID, merged.Clock.Sequence)
	}

	type mergeOutput struct {
		Report store.MergeReport `json:"report"`
		Merged *model.World      `json:"merged,omitempty"`
	}
	out := mergeOutput{Report: report}
	if !*apply {
		out.Merged = &merged
	}
	return writeJSON(stdout, stderr, "merge output", out)
}
