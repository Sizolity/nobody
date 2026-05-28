package devcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	case "debug-view":
		return runDebugView(ctx, args[1:], stdout, stderr)
	case "narrative-view":
		return runNarrativeView(ctx, args[1:], stdout, stderr)
	case "show":
		return runShow(ctx, args[1:], stdout, stderr)
	case "ingest-source":
		return runIngestSource(ctx, args[1:], stdout, stderr)
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

