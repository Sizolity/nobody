package devcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sizolity/nobody/internal/world/director"
	directorconfig "github.com/sizolity/nobody/internal/world/director/config"
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/runner"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/internal/world/store"
	worldview "github.com/sizolity/nobody/internal/world/view"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nobody-world <init|apply-event|step-script|step-reconcile|step-config|run|debug-view|narrative-view|show>")
		return 2
	}
	switch args[0] {
	case "init":
		return runInit(ctx, args[1:], stdout, stderr)
	case "apply-event":
		return runApplyEvent(ctx, args[1:], stdout, stderr)
	case "step-script":
		return runStepScript(ctx, args[1:], stdout, stderr)
	case "step-reconcile":
		return runStepReconcile(ctx, args[1:], stdout, stderr)
	case "step-config":
		return runStepConfig(ctx, args[1:], stdout, stderr)
	case "run":
		return runRun(ctx, args[1:], stdout, stderr)
	case "debug-view":
		return runDebugView(ctx, args[1:], stdout, stderr)
	case "narrative-view":
		return runNarrativeView(ctx, args[1:], stdout, stderr)
	case "show":
		return runShow(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	name := fs.String("name", "", "world name")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *name == "" {
		fmt.Fprintln(stderr, "init requires --workspace, --world-id, and --name")
		return 2
	}
	world := model.World{ID: model.WorldID(*worldID), Name: *name}
	if err := store.NewFileStore(*workspace).SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "init failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "created world %s\n", *worldID)
	return 0
}

func runApplyEvent(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("apply-event", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	eventFile := fs.String("event-file", "", "event JSON file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *eventFile == "" {
		fmt.Fprintln(stderr, "apply-event requires --workspace, --world-id, and --event-file")
		return 2
	}
	var event model.WorldEvent
	if err := readJSONFile(*eventFile, &event); err != nil {
		fmt.Fprintf(stderr, "read event failed: %v\n", err)
		return 1
	}
	r := runner.New(store.NewFileStore(*workspace))
	world, err := r.ApplyEvent(ctx, *worldID, event)
	if err != nil {
		fmt.Fprintf(stderr, "apply-event failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "applied event %s to world %s\n", event.ID, world.ID)
	return 0
}

func runStepScript(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("step-script", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	eventsFile := fs.String("events-file", "", "events JSON array file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *eventsFile == "" {
		fmt.Fprintln(stderr, "step-script requires --workspace, --world-id, and --events-file")
		return 2
	}
	var events []model.WorldEvent
	if err := readJSONFile(*eventsFile, &events); err != nil {
		fmt.Fprintf(stderr, "read events failed: %v\n", err)
		return 1
	}
	r := runner.New(
		store.NewFileStore(*workspace),
		worldruntime.WithDirectors(director.NewScriptDirector("cli_script", events)),
	)
	result, err := r.Step(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "step-script failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "applied %d events to world %s\n", len(result.AppliedEvents), result.World.ID)
	return 0
}

func runStepReconcile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("step-reconcile", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	casesFile := fs.String("cases-file", "", "reconcile cases JSON array file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *casesFile == "" {
		fmt.Fprintln(stderr, "step-reconcile requires --workspace, --world-id, and --cases-file")
		return 2
	}
	var cases []director.ReconcileCase
	if err := readJSONFile(*casesFile, &cases); err != nil {
		fmt.Fprintf(stderr, "read reconcile cases failed: %v\n", err)
		return 1
	}
	r := runner.New(
		store.NewFileStore(*workspace),
		worldruntime.WithDirectors(director.NewReconcileDirector("cli_reconcile", cases)),
	)
	result, err := r.Step(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "step-reconcile failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "applied %d reconciliation events to world %s\n", len(result.AppliedEvents), result.World.ID)
	return 0
}

func runStepConfig(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("step-config", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	configFile := fs.String("config-file", "", "director config JSON file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *configFile == "" {
		fmt.Fprintln(stderr, "step-config requires --workspace, --world-id, and --config-file")
		return 2
	}
	data, err := os.ReadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "read director config failed: %v\n", err)
		return 1
	}
	directors, err := directorconfig.LoadDirectors(data)
	if err != nil {
		fmt.Fprintf(stderr, "load director config failed: %v\n", err)
		return 1
	}
	r := runner.New(
		store.NewFileStore(*workspace),
		worldruntime.WithDirectors(directors...),
	)
	result, err := r.Step(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "step-config failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "applied %d configured events to world %s\n", len(result.AppliedEvents), result.World.ID)
	return 0
}

func runRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("run", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	configFile := fs.String("config-file", "", "director config JSON file")
	steps := fs.Int("steps", 1, "number of steps to execute")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *configFile == "" {
		fmt.Fprintln(stderr, "run requires --workspace, --world-id, and --config-file")
		return 2
	}
	data, err := os.ReadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "read director config failed: %v\n", err)
		return 1
	}
	directors, err := directorconfig.LoadDirectors(data)
	if err != nil {
		fmt.Fprintf(stderr, "load director config failed: %v\n", err)
		return 1
	}
	r := runner.New(
		store.NewFileStore(*workspace),
		worldruntime.WithDirectors(directors...),
	)
	result, err := r.Run(ctx, *worldID, *steps)
	if err != nil {
		fmt.Fprintf(stderr, "run failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "ran %d steps on world %s (%d events applied)\n",
		result.StepsCompleted, result.World.ID, len(result.AllAppliedEvents))
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

func writeJSON(stdout, stderr io.Writer, message string, v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", message, err)
		return 1
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(stderr, "write output failed: %v\n", err)
		return 1
	}
	return 0
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
