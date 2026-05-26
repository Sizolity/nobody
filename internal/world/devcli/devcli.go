package devcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

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
		fmt.Fprintln(stderr, "usage: nobody-world <init|apply-event|step-script|step-reconcile|step-config|step-llm|run|checkpoint|rollback|list-checkpoints|fork|beat|bridge-context|debug-view|narrative-view|show>")
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
	configFile := fs.String("config-file", "", "director config file (JSON or YAML)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *configFile == "" {
		fmt.Fprintln(stderr, "step-config requires --workspace, --world-id, and --config-file")
		return 2
	}
	directors, err := directorconfig.LoadDirectorsFromFile(*configFile, directorconfig.LoadOptions{
		GeneratorFactory: cliGeneratorFactory,
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
		GeneratorFactory: cliGeneratorFactory,
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
	WorldID          string                  `json:"world_id"`
	Plan             engine.BeatPlan         `json:"plan"`
	Draft            beatOutputDraft         `json:"draft"`
	ContinuityIssues []engine.ContinuityIssue `json:"continuity_issues"`
	Events           []beatOutputEvent       `json:"events"`
	Memories         []beatOutputMemory      `json:"memories"`
	Graph            beatOutputGraph         `json:"graph"`
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
	gen, err := cliGeneratorFactory(*provider, *modelName)
	if err != nil {
		fmt.Fprintf(stderr, "create generator: %v\n", err)
		return 1
	}

	var applyStore beatApplyStore
	if *apply {
		applyStore = fileStore
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
		bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{
			RecentEvents: *recentEvents,
			UserInput:    input,
		})

		result, err := executeBeatPipeline(ctx, gen, world, bundle, applyStore, *maxRewrites, stderr)
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

func executeBeatPipeline(ctx context.Context, gen engine.TextGenerator, world model.World, bundle engine.ContextBundle, applyStore beatApplyStore, maxRewrites int, stderr io.Writer) (beatPipelineResult, error) {
	directorAgent := engine.NewLLMDirectorAgent(gen)
	writerAgent := engine.NewLLMWriterAgent(gen)
	continuityAgent := engine.NewLLMContinuityAgent(gen)
	memoryAgent := engine.NewLLMMemoryAgent(gen)
	stateAgent := engine.NewLLMStateAgent(gen)

	plan, err := directorAgent.PlanBeat(ctx, bundle)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("director plan: %w", err)
	}
	fmt.Fprintf(stderr, "beat planned: %s → %s\n", plan.BeatID, plan.Objective)

	draft, err := writerAgent.WriteBeat(ctx, bundle, plan)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("writer draft: %w", err)
	}
	fmt.Fprintf(stderr, "draft written: %s (%s)\n", draft.Title, draft.Kind)

	report, err := continuityAgent.Check(ctx, bundle, draft)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("continuity check: %w", err)
	}

	for rewrite := 0; rewrite < maxRewrites && len(report.Issues) > 0; rewrite++ {
		fmt.Fprintf(stderr, "continuity: %d issue(s), rewriting (%d/%d)\n", len(report.Issues), rewrite+1, maxRewrites)
		draft, err = writerAgent.RewriteBeat(ctx, bundle, plan, draft, report.Issues)
		if err != nil {
			return beatPipelineResult{}, fmt.Errorf("rewrite %d: %w", rewrite+1, err)
		}
		fmt.Fprintf(stderr, "rewrite %d: %s (%s)\n", rewrite+1, draft.Title, draft.Kind)

		report, err = continuityAgent.Check(ctx, bundle, draft)
		if err != nil {
			return beatPipelineResult{}, fmt.Errorf("continuity re-check %d: %w", rewrite+1, err)
		}
	}

	if len(report.Issues) > 0 {
		fmt.Fprintf(stderr, "continuity: %d issue(s) remaining (proceeding)\n", len(report.Issues))
	} else {
		fmt.Fprintln(stderr, "continuity: clean")
	}

	memDelta, err := memoryAgent.Extract(ctx, bundle, draft)
	if err != nil {
		return beatPipelineResult{}, fmt.Errorf("memory extract: %w", err)
	}
	fmt.Fprintf(stderr, "memory: %d event(s), %d memory(ies)\n", len(memDelta.Events), len(memDelta.Memories))

	stateDelta, err := stateAgent.Apply(ctx, bundle, plan, memDelta)
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

func cliGeneratorFactory(provider, modelName string) (director.TextGenerator, error) {
	if provider == "" {
		provider = "deepseek"
	}
	envKey := providerEnvKey(provider)
	apiKey := os.Getenv(envKey)
	if apiKey == "" && envKey != "" {
		return nil, fmt.Errorf("%s environment variable is required for %s provider", envKey, provider)
	}
	return director.NewProviderGenerator(context.Background(), provider, modelName, apiKey)
}

func providerEnvKey(provider string) string {
	switch provider {
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	default:
		return ""
	}
}
