package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	htools "github.com/sizolity/nobody/internal/tools"
)

func TestEventLoggerWritesRuntimeJSONL(t *testing.T) {
	var _ htools.EventSink = (*EventLogger)(nil)
	dir := t.TempDir()
	logger := NewEventLogger(dir, "run-1")

	logger.Emit("runtime", "inference_ready", "info", "session-1", map[string]any{
		"provider": "llamacpp",
	})

	raw, err := os.ReadFile(filepath.Join(dir, "logs", "runtime.jsonl"))
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &record))
	require.Equal(t, "run-1", record["run_id"])
	require.Equal(t, "runtime", record["component"])
	require.Equal(t, "inference_ready", record["event"])
	require.Equal(t, "info", record["severity"])
	require.Equal(t, "session-1", record["session_id"])
	require.Equal(t, "llamacpp", record["payload"].(map[string]any)["provider"])
}

func TestEventLoggerRoutesUnknownComponentsToRuntimeLog(t *testing.T) {
	dir := t.TempDir()
	logger := NewEventLogger(dir, "run-1")

	logger.Emit("unknown", "something_happened", "warn", "", nil)

	_, err := os.Stat(filepath.Join(dir, "logs", "runtime.jsonl"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "logs", "unknown.jsonl"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestNewRunIDAndSanitizeRunIDForFilename(t *testing.T) {
	id := NewRunID()
	require.Regexp(t, `^run-\d{8}-\d{6}-[a-f0-9]{8}$`, id)
	require.Equal(t, id, SanitizeRunIDForFilename(id))
	require.Equal(t, "bad-run-id", SanitizeRunIDForFilename("../bad run id!!"))
	require.Equal(t, "run-unknown", SanitizeRunIDForFilename("..."))
}
