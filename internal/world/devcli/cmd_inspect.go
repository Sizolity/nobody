package devcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	worldview "github.com/sizolity/nobody/internal/world/view"
)

func runHistory(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("history", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	last := fs.Int("last", 0, "show only the last N events (0 = all)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "history requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	events := world.EventLog
	if *last > 0 && *last < len(events) {
		events = events[len(events)-*last:]
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "history output", store.BuildHistory(events))
	}

	entityNames := make(map[model.EntityID]string, len(world.Entities))
	for id, ent := range world.Entities {
		entityNames[id] = ent.Name
	}
	fmt.Fprint(stdout, store.FormatHistory(events, entityNames))
	return 0
}

func runStats(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("stats", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "stats requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	stats := store.ComputeStats(world)

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode stats", stats)
	}
	fmt.Fprint(stdout, store.FormatStats(stats))
	return 0
}

func runBudget(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("budget", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "budget requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	budget := store.EstimateBudget(world)

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode budget", budget)
	}
	fmt.Fprint(stdout, store.FormatBudget(budget))
	return 0
}

func runPreflight(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("preflight", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	maxTokens := fs.Int("max-tokens", 0, "token budget limit (0 = no limit)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "preflight requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	result := store.Preflight(world, *maxTokens)

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode preflight", result)
	}
	fmt.Fprint(stdout, store.FormatPreflight(result))
	if !result.Pass {
		return 1
	}
	return 0
}

func runInspectEntity(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("inspect-entity", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "entity id (omit to list all entities)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "inspect-entity requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	if *entityID == "" {
		if *format == "json" {
			return writeJSON(stdout, stderr, "encode entities", world.Entities)
		}
		fmt.Fprint(stdout, store.FormatEntityList(world))
		return 0
	}

	eid := model.EntityID(*entityID)
	entity, ok := world.Entities[eid]
	if !ok {
		fmt.Fprintf(stderr, "entity %q not found\n", *entityID)
		return 1
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode entity", entity)
	}
	fmt.Fprint(stdout, store.FormatEntity(entity, world))
	return 0
}

func runSummarize(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("summarize", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "summarize requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, store.FormatWorldSummary(world))
	return 0
}

func runShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("show", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "show requires --workspace and --world-id")
		return 2
	}
	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "show failed: %v\n", err)
		return 1
	}
	data, err := json.MarshalIndent(world, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode world failed: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(stderr, "write output failed: %v\n", err)
		return 1
	}
	return 0
}

func runDebugView(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("debug-view", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "debug-view requires --workspace and --world-id")
		return 2
	}
	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "debug-view failed: %v\n", err)
		return 1
	}
	return writeJSON(stdout, stderr, "encode debug view failed", worldview.WorldDebugView{}.Render(world))
}
