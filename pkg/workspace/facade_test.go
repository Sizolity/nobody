package workspace_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/workspace"
)

func TestPublicWorkspaceEventLoggerWritesRuntimeLog(t *testing.T) {
	dir := t.TempDir()
	logger := workspace.NewEventLogger(dir, "run-1")

	logger.Emit("runtime", "ready", "info", "", map[string]any{"provider": "llamacpp"})

	raw, err := os.ReadFile(filepath.Join(dir, "logs", "runtime.jsonl"))
	require.NoError(t, err)
	var record map[string]any
	require.NoError(t, json.Unmarshal(raw[:len(raw)-1], &record))
	require.Equal(t, "run-1", record["run_id"])
	require.Equal(t, "ready", record["event"])
}

func TestPublicWorkspaceRunMeta(t *testing.T) {
	runID := workspace.NewRunID()
	require.Equal(t, runID, workspace.SanitizeRunIDForFilename(runID))

	meta := workspace.RunMeta{
		SchemaVersion: workspace.RunMetaSchemaVersion,
		RunID:         runID,
		StartedAt:     time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 5, 19, 9, 1, 0, 0, time.UTC),
		Status:        workspace.RunStatusCompleted,
		Attempts:      1,
		EndReason:     "completed",
		TaskSummary:   "public workspace",
	}

	dir := t.TempDir()
	require.NoError(t, workspace.WriteRunMeta(filepath.Join(dir, "runs", runID), meta))
	require.NoError(t, workspace.AppendRunsIndex(filepath.Join(dir, "logs"), meta.IndexProjection(filepath.ToSlash(filepath.Join("runs", runID)))))
	require.Equal(t, "信号", workspace.TruncateRunes("信号城市", 2))
}
