package devcli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	bridgenarrative "github.com/sizolity/nobody/internal/bridge/narrative"
	"github.com/sizolity/nobody/internal/narrative/engine"
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
		fmt.Fprintln(stderr, "usage: nobody-world <init|apply-event|step-script|step-reconcile|step-config|step-llm|run|checkpoint|rollback|list-checkpoints|fork|lineage|diff|merge|validate|export|import|drain-queue|history|beat|bridge-context|debug-view|narrative-view|show>")
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
	case "step-llm":
		return runStepLLM(ctx, args[1:], stdout, stderr)
	case "run":
		return runRun(ctx, args[1:], stdout, stderr)
	case "checkpoint":
		return runCheckpoint(ctx, args[1:], stdout, stderr)
	case "rollback":
		return runRollback(ctx, args[1:], stdout, stderr)
	case "list-checkpoints":
		return runListCheckpoints(ctx, args[1:], stdout, stderr)
	case "fork":
		return runFork(ctx, args[1:], stdout, stderr)
	case "lineage":
		return runLineage(ctx, args[1:], stdout, stderr)
	case "diff":
		return runDiff(ctx, args[1:], stdout, stderr)
	case "merge":
		return runMerge(ctx, args[1:], stdout, stderr)
	case "validate":
		return runValidate(ctx, args[1:], stdout, stderr)
	case "export":
		return runExport(ctx, args[1:], stdout, stderr)
	case "import":
		return runImport(ctx, args[1:], stdout, stderr)
	case "drain-queue":
		return runDrainQueue(ctx, args[1:], stdout, stderr)
	case "history":
		return runHistory(ctx, args[1:], stdout, stderr)
	case "beat":
		return runBeat(ctx, args[1:], stdout, stderr)
	case "bridge-context":
		return runBridgeContext(ctx, args[1:], stdout, stderr)
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
	template := fs.String("template", "", "world template ("+strings.Join(store.TemplateNames(), ", ")+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *name == "" {
		fmt.Fprintln(stderr, "init requires --workspace, --world-id, and --name")
		return 2
	}

	var world model.World
	if *template != "" {
		tmpl, ok := store.Templates[*template]
		if !ok {
			fmt.Fprintf(stderr, "unknown template %q (available: %s)\n", *template, strings.Join(store.TemplateNames(), ", "))
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
		WorldID string          `json:"world_id"`
		Query   string          `json:"query"`
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
	resolveConflicts := fs.Bool("resolve-conflicts", false, "use LLM to resolve conflicts (requires --provider)")
	provider := fs.String("provider", "deepseek", "LLM provider for conflict resolution")
	modelName := fs.String("model", "", "model name for conflict resolution")
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

	type mergeResolutionOutput struct {
		Pick   string `json:"pick"`
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}

	var resolutionOutputs []mergeResolutionOutput

	if report.HasConflicts() && *resolveConflicts {
		gen, genErr := cliGeneratorFactory(*provider, *modelName)
		if genErr != nil {
			fmt.Fprintf(stderr, "create generator for conflict resolution: %v\n", genErr)
			return 1
		}
		resolver := store.NewLLMConflictResolver(gen)
		resolved, resolutions, resolveErr := store.ResolveMergeConflicts(ctx, merged, base, source, target, report.Conflicts, resolver)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "conflict resolution failed: %v\n", resolveErr)
			return 1
		}
		merged = resolved
		for i, res := range resolutions {
			fmt.Fprintf(stderr, "  resolved [%s] %s → %s (%s)\n", report.Conflicts[i].Kind, report.Conflicts[i].ID, res.Pick, res.Reason)
			resolutionOutputs = append(resolutionOutputs, mergeResolutionOutput{
				Pick: res.Pick, ID: report.Conflicts[i].ID, Kind: report.Conflicts[i].Kind, Reason: res.Reason,
			})
		}
		report.Conflicts = []store.MergeConflict{}
		fmt.Fprintf(stderr, "merge: all conflicts resolved\n")
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
		Report      store.MergeReport        `json:"report"`
		Resolutions []mergeResolutionOutput   `json:"resolutions,omitempty"`
		Merged      *model.World             `json:"merged,omitempty"`
	}
	out := mergeOutput{Report: report, Resolutions: resolutionOutputs}
	if !*apply {
		out.Merged = &merged
	}
	return writeJSON(stdout, stderr, "merge output", out)
}

func runValidate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
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

	if report.IsClean() {
		fmt.Fprintln(stderr, "validate: clean")
	} else {
		errors := report.ErrorCount()
		warnings := len(report.Issues) - errors
		fmt.Fprintf(stderr, "validate: %d error(s), %d warning(s)\n", errors, warnings)
	}

	code := writeJSON(stdout, stderr, "validate output", report)
	if code != 0 {
		return code
	}
	if report.ErrorCount() > 0 {
		return 1
	}
	return 0
}

func runExport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
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
			WorldID:  string(world.ID),
			Before:   0,
			After:    0,
			Applied:  []model.WorldEvent{},
			Skipped:  []model.WorldEvent{},
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

func runBridgeContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("bridge-context", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	recentEvents := fs.Int("recent-events", 0, "number of recent events to include; <=0 includes all")
	userInput := fs.String("input", "", "optional user input text")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "bridge-context requires --workspace and --world-id")
		return 2
	}
	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "bridge-context failed: %v\n", err)
		return 1
	}
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{
		RecentEvents: *recentEvents,
		UserInput:    *userInput,
	})
	return writeJSON(stdout, stderr, "encode bridge context failed", bundle)
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

type beatOutput struct {
	WorldID          string                   `json:"world_id"`
	Plan             engine.BeatPlan          `json:"plan"`
	Draft            beatOutputDraft          `json:"draft"`
	ContinuityIssues []engine.ContinuityIssue `json:"continuity_issues"`
	Events           []beatOutputEvent        `json:"events"`
	Memories         []beatOutputMemory       `json:"memories"`
	Graph            beatOutputGraph          `json:"graph"`
	Timing           []stageTiming            `json:"timing,omitempty"`
}

type stageTiming struct {
	Stage            string  `json:"stage"`
	Ms               float64 `json:"ms"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
}

// usageTracker is an optional interface for generators that report token usage.
type usageTracker interface {
	LastUsage() *director.TokenUsage
}

type beatOutputDraft struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
}

type beatOutputEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

type beatOutputMemory struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Subject    string `json:"subject"`
	Text       string `json:"text"`
	Importance int    `json:"importance"`
}

type beatOutputGraph struct {
	CurrentNodeID string `json:"current_node_id"`
	NodeCount     int    `json:"node_count"`
}

func runBeat(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("beat", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	provider := fs.String("provider", "deepseek", "LLM provider")
	modelName := fs.String("model", "", "model name (default per provider)")
	userInput := fs.String("input", "", "optional user input / prompt to steer the beat (first step only)")
	recentEvents := fs.Int("recent-events", 20, "max recent world events included in context")
	apply := fs.Bool("apply", false, "persist beat results back to world state")
	steps := fs.Int("steps", 1, "number of consecutive beats to run (implies --apply when > 1)")
	maxRewrites := fs.Int("max-rewrites", 2, "max continuity-driven rewrites per beat (0 to disable)")
	maxMemories := fs.Int("max-memories", 0, "max memories included in context (0 = all, sorted by importance)")
	interactive := fs.Bool("interactive", false, "pause after each pipeline stage for user review")
	stream := fs.Bool("stream", false, "stream LLM output chunks to stderr in real-time")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "beat requires --workspace and --world-id")
		return 2
	}
	if *steps < 1 {
		fmt.Fprintln(stderr, "beat --steps must be >= 1")
		return 2
	}
	if *steps > 1 {
		*apply = true
	}

	fileStore := store.NewFileStore(*workspace)
	var genOpts []director.ProviderGeneratorOption
	if *stream {
		genOpts = append(genOpts, director.WithStreamWriter(stderr))
	}
	gen, err := cliGeneratorFactory(*provider, *modelName, genOpts...)
	if err != nil {
		fmt.Fprintf(stderr, "create generator: %v\n", err)
		return 1
	}

	var applyStore beatApplyStore
	if *apply {
		applyStore = fileStore
	}

	var hook PipelineHook
	if *interactive {
		hook = newStdinHook(os.Stdin, stderr)
	}

	results := make([]beatOutput, 0, *steps)
	for step := 0; step < *steps; step++ {
		if *steps > 1 {
			fmt.Fprintf(stderr, "\n=== beat %d/%d ===\n", step+1, *steps)
		}

		world, err := fileStore.LoadSnapshot(ctx, *worldID)
		if err != nil {
			fmt.Fprintf(stderr, "load world: %v\n", err)
			return 1
		}

		input := ""
		if step == 0 {
			input = *userInput
		}
		adaptOpts := bridgenarrative.Options{
			RecentEvents: *recentEvents,
			UserInput:    input,
		}
		if *maxMemories > 0 {
			adaptOpts.MemoryFilter = &store.MemoryFilter{MaxCount: *maxMemories}
		}
		bundle := bridgenarrative.AdaptWorld(world, adaptOpts)

		result, err := executeBeatPipelineWithHook(ctx, gen, world, bundle, beatPipelineOpts{
			applyStore: applyStore, maxRewrites: *maxRewrites, hook: hook,
		}, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		results = append(results, result.Output)
	}

	if len(results) == 1 {
		return writeJSON(stdout, stderr, "encode beat output failed", results[0])
	}
	return writeJSON(stdout, stderr, "encode beat output failed", results)
}

type beatApplyStore interface {
	SaveSnapshot(ctx context.Context, world model.World) error
}

type beatPipelineResult struct {
	Output  beatOutput
	Updated model.World
}

// PipelineHook is called after each pipeline stage in interactive mode.
// It receives the stage name and a human-readable summary of the stage output.
// Return values:
//   - feedback: optional text appended to the next LLM call's context
//   - abort: if true, the pipeline stops with a user-abort error
type PipelineHook interface {
	AfterStage(stage, summary string) (feedback string, abort bool)
}

type beatPipelineOpts struct {
	applyStore  beatApplyStore
	maxRewrites int
	hook        PipelineHook
}

func executeBeatPipeline(ctx context.Context, gen engine.TextGenerator, world model.World, bundle engine.ContextBundle, applyStore beatApplyStore, maxRewrites int, stderr io.Writer) (beatPipelineResult, error) {
	return executeBeatPipelineWithHook(ctx, gen, world, bundle, beatPipelineOpts{
		applyStore: applyStore, maxRewrites: maxRewrites,
	}, stderr)
}

func executeBeatPipelineWithHook(ctx context.Context, gen engine.TextGenerator, world model.World, bundle engine.ContextBundle, opts beatPipelineOpts, stderr io.Writer) (beatPipelineResult, error) {
	applyStore := opts.applyStore
	maxRewrites := opts.maxRewrites
	hook := opts.hook
	directorAgent := engine.NewLLMDirectorAgent(gen)
	writerAgent := engine.NewLLMWriterAgent(gen)
	continuityAgent := engine.NewLLMContinuityAgent(gen)
	memoryAgent := engine.NewLLMMemoryAgent(gen)
	stateAgent := engine.NewLLMStateAgent(gen)

	ut, _ := gen.(usageTracker)

	var timings []stageTiming
	trackTime := func(stage string, start time.Time) {
		st := stageTiming{
			Stage: stage,
			Ms:    float64(time.Since(start).Microseconds()) / 1000.0,
		}
		if ut != nil {
			if u := ut.LastUsage(); u != nil {
				st.PromptTokens = u.PromptTokens
				st.CompletionTokens = u.CompletionTokens
				st.TotalTokens = u.TotalTokens
			}
		}
		timings = append(timings, st)
	}

	t0 := time.Now()
	plan, err := directorAgent.PlanBeat(ctx, bundle)
	trackTime("plan", t0)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("director plan: %w", err)
	}
	fmt.Fprintf(stderr, "beat planned: %s → %s\n", plan.BeatID, plan.Objective)

	if hook != nil {
		feedback, abort := hook.AfterStage("plan", fmt.Sprintf("Beat %s: %s (target: %s)", plan.BeatID, plan.Objective, plan.TargetNodeID))
		if abort {
			return beatPipelineResult{}, fmt.Errorf("aborted by user after plan")
		}
		if feedback != "" {
			bundle.Input = feedback
		}
	}

	t0 = time.Now()
	draft, err := writerAgent.WriteBeat(ctx, bundle, plan)
	trackTime("write", t0)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("writer draft: %w", err)
	}
	fmt.Fprintf(stderr, "draft written: %s (%s)\n", draft.Title, draft.Kind)

	if hook != nil {
		feedback, abort := hook.AfterStage("draft", fmt.Sprintf("%s\n\n%s", draft.Title, draft.Text))
		if abort {
			return beatPipelineResult{}, fmt.Errorf("aborted by user after draft")
		}
		if feedback != "" {
			bundle.Input = feedback
		}
	}

	t0 = time.Now()
	report, err := continuityAgent.Check(ctx, bundle, draft)
	trackTime("continuity", t0)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("continuity check: %w", err)
	}

	for rewrite := 0; rewrite < maxRewrites && len(report.Issues) > 0; rewrite++ {
		fmt.Fprintf(stderr, "continuity: %d issue(s), rewriting (%d/%d)\n", len(report.Issues), rewrite+1, maxRewrites)
		t0 = time.Now()
		draft, err = writerAgent.RewriteBeat(ctx, bundle, plan, draft, report.Issues)
		trackTime(fmt.Sprintf("rewrite_%d", rewrite+1), t0)
		if err != nil {
			return beatPipelineResult{}, fmt.Errorf("rewrite %d: %w", rewrite+1, err)
		}
		fmt.Fprintf(stderr, "rewrite %d: %s (%s)\n", rewrite+1, draft.Title, draft.Kind)

		t0 = time.Now()
		report, err = continuityAgent.Check(ctx, bundle, draft)
		trackTime(fmt.Sprintf("recheck_%d", rewrite+1), t0)
		if err != nil {
			return beatPipelineResult{}, fmt.Errorf("continuity re-check %d: %w", rewrite+1, err)
		}
	}

	if len(report.Issues) > 0 {
		fmt.Fprintf(stderr, "continuity: %d issue(s) remaining", len(report.Issues))
		if report.HasCritical() {
			fmt.Fprint(stderr, " (includes critical)")
		}
		fmt.Fprintln(stderr)
	} else {
		fmt.Fprintln(stderr, "continuity: clean")
	}

	if hook != nil {
		summary := "continuity: clean"
		if len(report.Issues) > 0 {
			summary = fmt.Sprintf("continuity: %d issue(s) remaining", len(report.Issues))
			for _, issue := range report.Issues {
				summary += fmt.Sprintf("\n  [%s] %s: %s", issue.Severity, issue.Code, issue.Summary)
			}
		}
		feedback, abort := hook.AfterStage("continuity", summary)
		if abort {
			return beatPipelineResult{}, fmt.Errorf("aborted by user after continuity check")
		}
		if feedback != "" {
			bundle.Input = feedback
		}
	}

	if applyStore != nil && report.HasCritical() {
		return beatPipelineResult{}, fmt.Errorf("continuity: critical issues remain after %d rewrite(s); refusing to apply", maxRewrites)
	}

	t0 = time.Now()
	memDelta, err := memoryAgent.Extract(ctx, bundle, draft)
	trackTime("memory", t0)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("memory extract: %w", err)
	}
	fmt.Fprintf(stderr, "memory: %d event(s), %d memory(ies)\n", len(memDelta.Events), len(memDelta.Memories))

	t0 = time.Now()
	stateDelta, err := stateAgent.Apply(ctx, bundle, plan, memDelta)
	trackTime("state", t0)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("state update: %w", err)
	}
	fmt.Fprintf(stderr, "graph: %d node(s), current=%s\n", len(stateDelta.Graph.Nodes), stateDelta.Graph.CurrentNodeID)

	updated := bridgenarrative.ApplyBeatResult(world, bridgenarrative.BeatResult{
		Plan: plan, Draft: draft, Report: report,
		MemDelta: memDelta, StateDelta: stateDelta,
	})

	if applyStore != nil {
		if err := applyStore.SaveSnapshot(ctx, updated); err != nil {
			return beatPipelineResult{}, fmt.Errorf("apply failed: %w", err)
		}
		fmt.Fprintf(stderr, "applied: world %s updated (sequence %d)\n", updated.ID, updated.Clock.Sequence)
	}

	var totalMs float64
	var totalPrompt, totalCompletion, totalTokens int
	for _, t := range timings {
		totalMs += t.Ms
		totalPrompt += t.PromptTokens
		totalCompletion += t.CompletionTokens
		totalTokens += t.TotalTokens
	}
	fmt.Fprintf(stderr, "timing:")
	for _, t := range timings {
		fmt.Fprintf(stderr, " %s=%.0fms", t.Stage, t.Ms)
	}
	fmt.Fprintf(stderr, " total=%.0fms\n", totalMs)
	if totalTokens > 0 {
		fmt.Fprintf(stderr, "tokens: prompt=%d completion=%d total=%d\n", totalPrompt, totalCompletion, totalTokens)
	}

	events := make([]beatOutputEvent, 0, len(memDelta.Events))
	for _, ev := range memDelta.Events {
		events = append(events, beatOutputEvent{
			ID: ev.ID, Type: ev.Type, Summary: ev.Summary,
		})
	}
	memories := make([]beatOutputMemory, 0, len(memDelta.Memories))
	for _, m := range memDelta.Memories {
		memories = append(memories, beatOutputMemory{
			ID: m.ID, Type: m.Type, Subject: m.Subject, Text: m.Text, Importance: m.Importance,
		})
	}

	return beatPipelineResult{
		Output: beatOutput{
			WorldID:          string(world.ID),
			Plan:             plan,
			Draft:            beatOutputDraft{ID: draft.ID, Title: draft.Title, Kind: draft.Kind, Text: draft.Text},
			ContinuityIssues: report.Issues,
			Events:           events,
			Memories:         memories,
			Graph:            beatOutputGraph{CurrentNodeID: stateDelta.Graph.CurrentNodeID, NodeCount: len(stateDelta.Graph.Nodes)},
			Timing:           timings,
		},
		Updated: updated,
	}, nil
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

type stdinHook struct {
	scanner *bufio.Scanner
	stderr  io.Writer
}

func newStdinHook(stdin io.Reader, stderr io.Writer) *stdinHook {
	return &stdinHook{scanner: bufio.NewScanner(stdin), stderr: stderr}
}

func (h *stdinHook) AfterStage(stage, summary string) (string, bool) {
	fmt.Fprintf(h.stderr, "\n--- %s ---\n%s\n", stage, summary)
	fmt.Fprintf(h.stderr, "[Enter=continue, 'abort'=stop, or type feedback]: ")
	if !h.scanner.Scan() {
		return "", true
	}
	line := strings.TrimSpace(h.scanner.Text())
	if strings.EqualFold(line, "abort") {
		return "", true
	}
	return line, false
}

// configGeneratorFactory matches the directorconfig.GeneratorFactory signature
// (no variadic opts). Used by step-config and run where streaming is not needed.
func configGeneratorFactory(provider, modelName string) (director.TextGenerator, error) {
	return cliGeneratorFactory(provider, modelName)
}

func cliGeneratorFactory(provider, modelName string, opts ...director.ProviderGeneratorOption) (director.TextGenerator, error) {
	if provider == "" {
		provider = "deepseek"
	}
	envKey := providerEnvKey(provider)
	apiKey := os.Getenv(envKey)
	if apiKey == "" && envKey != "" {
		return nil, fmt.Errorf("%s environment variable is required for %s provider", envKey, provider)
	}
	return director.NewProviderGenerator(context.Background(), provider, modelName, apiKey, opts...)
}

func providerEnvKey(provider string) string {
	switch provider {
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	default:
		return ""
	}
}
