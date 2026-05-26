package devcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	rpgbridge "github.com/sizolity/nobody/rpg/bridge"
	"github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/internal/world/director"
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
	bundle := rpgbridge.AdaptWorld(world, rpgbridge.Options{
		RecentEvents: *recentEvents,
		UserInput:    *userInput,
	})
	return writeJSON(stdout, stderr, "encode bridge context failed", bundle)
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

func formatBeatOutput(o beatOutput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Beat — %s\n\n", o.WorldID)

	fmt.Fprintf(&b, "## Plan\n\n")
	fmt.Fprintf(&b, "- **Beat ID**: %s\n", o.Plan.BeatID)
	fmt.Fprintf(&b, "- **Objective**: %s\n", o.Plan.Objective)
	if o.Plan.TargetNodeID != "" {
		fmt.Fprintf(&b, "- **Target Node**: %s\n", o.Plan.TargetNodeID)
	}
	fmt.Fprintln(&b)

	if o.Draft.Title != "" {
		fmt.Fprintf(&b, "## Draft: %s\n\n", o.Draft.Title)
		if o.Draft.Kind != "" {
			fmt.Fprintf(&b, "*Kind: %s*\n\n", o.Draft.Kind)
		}
		text := o.Draft.Text
		if len(text) > 500 {
			text = text[:497] + "..."
		}
		fmt.Fprintf(&b, "%s\n\n", text)
	}

	if len(o.Events) > 0 {
		fmt.Fprintf(&b, "## Events (%d)\n\n", len(o.Events))
		for _, e := range o.Events {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", e.ID, e.Type, e.Summary)
		}
		fmt.Fprintln(&b)
	}

	if len(o.Memories) > 0 {
		fmt.Fprintf(&b, "## Memories (%d)\n\n", len(o.Memories))
		for _, m := range o.Memories {
			line := fmt.Sprintf("- [%s] %s", m.ID, m.Text)
			if m.Subject != "" {
				line += fmt.Sprintf(" (subject=%s)", m.Subject)
			}
			if m.Importance > 0 {
				line += fmt.Sprintf(" importance=%d", m.Importance)
			}
			fmt.Fprintln(&b, line)
		}
		fmt.Fprintln(&b)
	}

	if len(o.ContinuityIssues) > 0 {
		fmt.Fprintf(&b, "## Continuity Issues (%d)\n\n", len(o.ContinuityIssues))
		for _, ci := range o.ContinuityIssues {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", ci.Severity, ci.Code, ci.Summary)
		}
		fmt.Fprintln(&b)
	}

	if o.Graph.NodeCount > 0 {
		fmt.Fprintf(&b, "## Story Graph\n\n")
		fmt.Fprintf(&b, "- **Current Node**: %s\n", o.Graph.CurrentNodeID)
		fmt.Fprintf(&b, "- **Total Nodes**: %d\n\n", o.Graph.NodeCount)
	}

	if len(o.Timing) > 0 {
		fmt.Fprintf(&b, "## Timing\n\n")
		var totalMs float64
		var totalPrompt, totalCompletion, totalTokens int
		for _, t := range o.Timing {
			line := fmt.Sprintf("- %s: %.0fms", t.Stage, t.Ms)
			if t.TotalTokens > 0 {
				line += fmt.Sprintf(" (%d tokens)", t.TotalTokens)
			}
			fmt.Fprintln(&b, line)
			totalMs += t.Ms
			totalPrompt += t.PromptTokens
			totalCompletion += t.CompletionTokens
			totalTokens += t.TotalTokens
		}
		fmt.Fprintf(&b, "\n**Total**: %.0fms", totalMs)
		if totalTokens > 0 {
			fmt.Fprintf(&b, " | %d tokens (prompt=%d, completion=%d)", totalTokens, totalPrompt, totalCompletion)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
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
	planOnly := fs.Bool("plan-only", false, "run only the plan stage (1 LLM call) and output the beat plan")
	interactive := fs.Bool("interactive", false, "pause after each pipeline stage for user review")
	stream := fs.Bool("stream", false, "stream LLM output chunks to stderr in real-time")
	format := fs.String("format", "json", "output format: json or text")
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
		adaptOpts := rpgbridge.Options{
			RecentEvents: *recentEvents,
			UserInput:    input,
		}
		if *maxMemories > 0 {
			adaptOpts.MemoryFilter = &store.MemoryFilter{MaxCount: *maxMemories}
		}
		bundle := rpgbridge.AdaptWorld(world, adaptOpts)

		result, err := executeBeatPipelineWithHook(ctx, gen, world, bundle, beatPipelineOpts{
			applyStore: applyStore, maxRewrites: *maxRewrites, hook: hook, planOnly: *planOnly,
		}, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		results = append(results, result.Output)
	}

	if *format == "text" {
		for i, r := range results {
			if len(results) > 1 {
				fmt.Fprintf(stdout, "=== Beat %d/%d ===\n\n", i+1, len(results))
			}
			fmt.Fprint(stdout, formatBeatOutput(r))
		}
		return 0
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
	planOnly    bool
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

	if opts.planOnly {
		return beatPipelineResult{
			Output: beatOutput{
				WorldID: string(world.ID),
				Plan:    plan,
				Timing:  timings,
			},
		}, nil
	}

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

	updated := rpgbridge.ApplyBeatResult(world, rpgbridge.BeatResult{
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
