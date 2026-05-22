// Package workspace exposes product-neutral workspace observability helpers for
// downstream product repositories.
package workspace

import internal "github.com/sizolity/nobody/internal/workspace"

type EventLogger = internal.EventLogger
type RunMeta = internal.RunMeta
type RunIndexLine = internal.RunIndexLine

const (
	RunMetaSchemaVersion = internal.RunMetaSchemaVersion
	RunStatusCompleted   = internal.RunStatusCompleted
	RunStatusInProgress  = internal.RunStatusInProgress
	RunStatusAborted     = internal.RunStatusAborted
	TaskSummaryMaxRunes  = internal.TaskSummaryMaxRunes
	ErrorMaxRunes        = internal.ErrorMaxRunes
)

func NewEventLogger(workspace, runID string) *EventLogger {
	return internal.NewEventLogger(workspace, runID)
}

func NewRunID() string {
	return internal.NewRunID()
}

func SanitizeRunIDForFilename(id string) string {
	return internal.SanitizeRunIDForFilename(id)
}

func TruncateRunes(s string, n int) string {
	return internal.TruncateRunes(s, n)
}

func WriteRunMeta(dir string, meta RunMeta) error {
	return internal.WriteRunMeta(dir, meta)
}

func AppendRunsIndex(logsDir string, line RunIndexLine) error {
	return internal.AppendRunsIndex(logsDir, line)
}
