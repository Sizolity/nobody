package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteRunMetaWritesAtomicJSON(t *testing.T) {
	dir := t.TempDir()
	meta := RunMeta{
		SchemaVersion: RunMetaSchemaVersion,
		RunID:         "run-1",
		StartedAt:     time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 5, 19, 9, 1, 0, 0, time.UTC),
		Status:        RunStatusCompleted,
		Attempts:      1,
		EndReason:     "completed",
		TaskSummary:   "test run",
	}

	require.NoError(t, WriteRunMeta(dir, meta))

	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	require.NoError(t, err)
	var got RunMeta
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, meta, got)
}

func TestAppendRunsIndexAppendsJSONLProjection(t *testing.T) {
	logsDir := t.TempDir()
	meta := RunMeta{
		SchemaVersion: RunMetaSchemaVersion,
		RunID:         "run-1",
		StartedAt:     time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 5, 19, 9, 1, 0, 0, time.UTC),
		Status:        RunStatusAborted,
		Attempts:      2,
		TaskSummary:   "failed run",
		Error:         "details stay in meta.json",
	}

	require.NoError(t, AppendRunsIndex(logsDir, meta.IndexProjection("runs/run-1")))

	raw, err := os.ReadFile(filepath.Join(logsDir, "runs-index.jsonl"))
	require.NoError(t, err)
	var got RunIndexLine
	require.NoError(t, json.Unmarshal(raw[:len(raw)-1], &got))
	require.Equal(t, "run-1", got.RunID)
	require.Equal(t, RunStatusAborted, got.Status)
	require.Equal(t, "runs/run-1", got.Dir)
}

func TestTruncateRunesIsRuneAware(t *testing.T) {
	require.Equal(t, "信号", TruncateRunes("信号城市", 2))
	require.Equal(t, "Signal", TruncateRunes("Signal", 0))
}
