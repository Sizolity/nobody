package devcli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sizolity/nobody/internal/world/director"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nobody-world <command>")
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
	case "summarize":
		return runSummarize(ctx, args[1:], stdout, stderr)
	case "inspect-entity":
		return runInspectEntity(ctx, args[1:], stdout, stderr)
	case "manage-entity":
		return runManageEntity(ctx, args[1:], stdout, stderr)
	case "manage-thread":
		return runManageThread(ctx, args[1:], stdout, stderr)
	case "manage-memory":
		return runManageMemory(ctx, args[1:], stdout, stderr)
	case "clock":
		return runClock(ctx, args[1:], stdout, stderr)
	case "stats":
		return runStats(ctx, args[1:], stdout, stderr)
	case "budget":
		return runBudget(ctx, args[1:], stdout, stderr)
	case "preflight":
		return runPreflight(ctx, args[1:], stdout, stderr)
	case "manage-relation":
		return runManageRelation(ctx, args[1:], stdout, stderr)
	case "manage-fact":
		return runManageFact(ctx, args[1:], stdout, stderr)
	case "manage-queue":
		return runManageQueue(ctx, args[1:], stdout, stderr)
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

// --- shared helpers ---

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
