package devcli

import (
	"context"
	"fmt"
	"io"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/internal/world/store"
	worldview "github.com/sizolity/nobody/internal/world/view"
)

func runDrainQueue(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("drain-queue", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	limit := fs.Int("limit", 0, "max queued events to process (0 = all)")
	dryRun := fs.Bool("dry-run", false, "show what would be processed without saving")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "drain-queue requires --workspace and --world-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	queueLen := len(world.EventQueue)
	if queueLen == 0 {
		fmt.Fprintln(stderr, "drain-queue: queue is empty")
		return writeJSON(stdout, stderr, "drain-queue output", drainQueueOutput{
			WorldID: string(world.ID),
			Before:  0,
			After:   0,
			Applied: []model.WorldEvent{},
			Skipped: []model.WorldEvent{},
		})
	}

	processLimit := queueLen
	if *limit > 0 && *limit < processLimit {
		processLimit = *limit
	}

	rt := worldruntime.NewRuntime(worldruntime.WithEventQueueLimit(processLimit))
	result, err := rt.Step(ctx, world)
	if err != nil {
		fmt.Fprintf(stderr, "drain-queue: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "drain-queue: %d applied, %d skipped, %d remaining\n",
		len(result.AppliedEvents), len(result.SkippedEvents), len(result.World.EventQueue))

	if !*dryRun {
		if err := fileStore.SaveSnapshot(ctx, result.World); err != nil {
			fmt.Fprintf(stderr, "save world: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "drain-queue: saved (sequence %d)\n", result.World.Clock.Sequence)
	} else {
		fmt.Fprintln(stderr, "drain-queue: dry-run, not saved")
	}

	return writeJSON(stdout, stderr, "drain-queue output", drainQueueOutput{
		WorldID:  string(result.World.ID),
		Before:   queueLen,
		After:    len(result.World.EventQueue),
		Applied:  result.AppliedEvents,
		Skipped:  result.SkippedEvents,
		Sequence: result.World.Clock.Sequence,
	})
}

type drainQueueOutput struct {
	WorldID  string             `json:"world_id"`
	Before   int                `json:"queue_before"`
	After    int                `json:"queue_after"`
	Applied  []model.WorldEvent `json:"applied"`
	Skipped  []model.WorldEvent `json:"skipped"`
	Sequence int64              `json:"sequence"`
}

func runNarrativeView(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("narrative-view", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	recentEvents := fs.Int("recent-events", 0, "number of recent events to include; <=0 includes all")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "narrative-view requires --workspace and --world-id")
		return 2
	}
	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "narrative-view failed: %v\n", err)
		return 1
	}
	ctxView := worldview.NarrativeView{}.Render(world, worldview.NarrativeContextRequest{RecentEventLimit: *recentEvents})
	return writeJSON(stdout, stderr, "encode narrative view failed", ctxView)
}
