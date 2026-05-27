package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

const (
	// RunMetaSchemaVersion is the schema discriminator written into every
	// runs/<id>/meta.json and logs/runs-index.jsonl line. Bump on breaking
	// schema changes.
	RunMetaSchemaVersion = 1

	RunStatusCompleted  = "COMPLETED"
	RunStatusInProgress = "IN_PROGRESS"
	RunStatusAborted    = "ABORTED"

	TaskSummaryMaxRunes = 80
	ErrorMaxRunes       = 200
)

// RunMeta is the canonical per-run summary written to runs/<run-id>/meta.json.
// It is the authoritative source; logs/runs-index.jsonl is a projection of a
// subset of these fields.
type RunMeta struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	EndReason     string    `json:"end_reason"`
	TaskSummary   string    `json:"task_summary"`
	Error         string    `json:"error,omitempty"`
}

type RunIndexLine struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	TaskSummary   string    `json:"task_summary"`
	Dir           string    `json:"dir"`
}

func (m RunMeta) IndexProjection(dir string) RunIndexLine {
	return RunIndexLine{
		SchemaVersion: m.SchemaVersion,
		RunID:         m.RunID,
		StartedAt:     m.StartedAt,
		FinishedAt:    m.FinishedAt,
		Status:        m.Status,
		Attempts:      m.Attempts,
		TaskSummary:   m.TaskSummary,
		Dir:           dir,
	}
}

func TruncateRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	out := make([]rune, 0, n)
	for _, r := range s {
		if len(out) == n {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

func WriteRunMeta(dir string, m RunMeta) error {
	if dir == "" {
		return fmt.Errorf("run_meta: dir must be non-empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("run_meta: mkdir run dir: %w", err)
	}
	final := filepath.Join(dir, "meta.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("run_meta: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".meta-*.json")
	if err != nil {
		return fmt.Errorf("run_meta: create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, werr := tmp.Write(data); werr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("run_meta: write tmp: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("run_meta: close tmp: %w", cerr)
	}
	if rerr := os.Rename(tmpPath, final); rerr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("run_meta: rename: %w", rerr)
	}
	return nil
}

func AppendRunsIndex(logsDir string, line RunIndexLine) error {
	if logsDir == "" {
		return fmt.Errorf("run_meta: logsDir must be non-empty")
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("run_meta: mkdir logs: %w", err)
	}
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("run_meta: marshal index: %w", err)
	}
	path := filepath.Join(logsDir, "runs-index.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("run_meta: open index: %w", err)
	}
	defer f.Close()
	buf := append(data, '\n')
	if _, werr := f.Write(buf); werr != nil {
		return fmt.Errorf("run_meta: write index: %w", werr)
	}
	return nil
}
