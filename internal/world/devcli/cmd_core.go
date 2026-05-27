package devcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sizolity/nobody/internal/world/director"
	directorconfig "github.com/sizolity/nobody/internal/world/director/config"
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/runner"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/internal/world/store"
	rpgtemplate "github.com/sizolity/nobody/rpg/template"
)

func runInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	name := fs.String("name", "", "world name")
	template := fs.String("template", "", "world template ("+strings.Join(rpgtemplate.TemplateNames(), ", ")+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *name == "" {
		fmt.Fprintln(stderr, "init requires --workspace, --world-id, and --name")
		return 2
	}

	var world model.World
	if *template != "" {
		tmpl, ok := rpgtemplate.Templates[*template]
		if !ok {
			fmt.Fprintf(stderr, "unknown template %q (available: %s)\n", *template, strings.Join(rpgtemplate.TemplateNames(), ", "))
			return 2
		}
		var err error
		world, err = store.ApplyTemplate(tmpl, *worldID, *name)
		if err != nil {
			fmt.Fprintf(stderr, "apply template: %v\n", err)
			return 1
		}
	} else {
		world = model.World{ID: model.WorldID(*worldID), Name: *name}
	}

	if err := store.NewFileStore(*workspace).SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "init failed: %v\n", err)
		return 1
	}
	entityCount := len(world.Entities)
	if entityCount > 0 {
		fmt.Fprintf(stdout, "created world %s from template %q (%d entities, %d facts, %d threads)\n",
			*worldID, *template, entityCount, len(world.Facts), len(world.Threads))
	} else {
		fmt.Fprintf(stdout, "created world %s\n", *worldID)
	}
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
	configFile := fs.String("config-file", "", "director config file (JSON or YAML)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *configFile == "" {
		fmt.Fprintln(stderr, "step-config requires --workspace, --world-id, and --config-file")
		return 2
	}
	directors, err := directorconfig.LoadDirectorsFromFile(*configFile, directorconfig.LoadOptions{
		GeneratorFactory: configGeneratorFactory,
	})
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

func runStepLLM(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("step-llm", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	provider := fs.String("provider", "deepseek", "LLM provider (deepseek)")
	modelName := fs.String("model", "", "model name (default per provider)")
	systemPrompt := fs.String("system-prompt", "", "system prompt for the LLM director")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "step-llm requires --workspace and --world-id")
		return 2
	}
	gen, err := cliGeneratorFactory(*provider, *modelName)
	if err != nil {
		fmt.Fprintf(stderr, "create generator failed: %v\n", err)
		return 1
	}
	d := director.NewLLMDirector("cli_llm", director.LLMDirectorConfig{
		SystemPrompt: *systemPrompt,
		Generator:    gen,
	})
	r := runner.New(
		store.NewFileStore(*workspace),
		worldruntime.WithDirectors(d),
	)
	result, err := r.Step(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "step-llm failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "applied %d LLM events to world %s\n", len(result.AppliedEvents), result.World.ID)
	return 0
}

func runRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("run", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	configFile := fs.String("config-file", "", "director config file (JSON or YAML)")
	steps := fs.Int("steps", 1, "number of steps to execute")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *configFile == "" {
		fmt.Fprintln(stderr, "run requires --workspace, --world-id, and --config-file")
		return 2
	}
	directors, err := directorconfig.LoadDirectorsFromFile(*configFile, directorconfig.LoadOptions{
		GeneratorFactory: configGeneratorFactory,
	})
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

func runValidate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "validate requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	report := store.DeepValidate(world)

	errors := report.ErrorCount()
	warnings := len(report.Issues) - errors
	if report.IsClean() {
		fmt.Fprintln(stderr, "validate: clean")
	} else {
		fmt.Fprintf(stderr, "validate: %d error(s), %d warning(s)\n", errors, warnings)
	}

	if *format == "json" {
		code := writeJSON(stdout, stderr, "validate output", report)
		if code != 0 {
			return code
		}
	} else {
		fmt.Fprint(stdout, store.FormatValidationReport(report))
	}

	if report.ErrorCount() > 0 {
		return 1
	}
	return 0
}

func runExport(ctx context.Context, args []string, _, stderr io.Writer) int {
	fs := newFlagSet("export", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	output := fs.String("output", "", "output file path (tar.gz)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *output == "" {
		fmt.Fprintln(stderr, "export requires --workspace, --world-id, and --output")
		return 2
	}

	f, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(stderr, "create output file: %v\n", err)
		return 1
	}
	defer f.Close()

	if err := store.ExportToFileStore(ctx, store.NewFileStore(*workspace), *worldID, f); err != nil {
		fmt.Fprintf(stderr, "export: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "exported world %s to %s\n", *worldID, *output)
	return 0
}

func runImport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("import", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	input := fs.String("input", "", "input file path (tar.gz)")
	newID := fs.String("new-id", "", "optional new world id (replaces the archived id)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *input == "" {
		fmt.Fprintln(stderr, "import requires --workspace and --input")
		return 2
	}

	f, err := os.Open(*input)
	if err != nil {
		fmt.Fprintf(stderr, "open input file: %v\n", err)
		return 1
	}
	defer f.Close()

	world, err := store.ImportToFileStore(ctx, store.NewFileStore(*workspace), f, *newID)
	if err != nil {
		fmt.Fprintf(stderr, "import: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "imported world %s (sequence %d)\n", world.ID, world.Clock.Sequence)
	return writeJSON(stdout, stderr, "import output", map[string]any{
		"world_id": world.ID,
		"name":     world.Name,
		"sequence": world.Clock.Sequence,
	})
}
