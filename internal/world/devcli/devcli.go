package devcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/runner"
	"github.com/sizolity/nobody/internal/world/store"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nobody-world <init|apply-event|show>")
		return 2
	}
	switch args[0] {
	case "init":
		return runInit(ctx, args[1:], stdout, stderr)
	case "apply-event":
		return runApplyEvent(ctx, args[1:], stdout, stderr)
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
