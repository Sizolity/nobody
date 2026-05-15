package harness

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	htools "github.com/sizolity/nobody/internal/tools"
)

type EventLogger struct {
	dir   string
	runID string
	mu    sync.Mutex
}

// Compile-time assertion that *EventLogger satisfies tools.EventSink.
// The two types are kept in lock-step on purpose so tool runtimes can
// emit runtime events without importing this package (which would
// create an import cycle). If Emit's signature ever drifts, this line
// catches it at build time instead of at a harness wiring site.
var _ htools.EventSink = (*EventLogger)(nil)

func NewEventLogger(workspace, runID string) *EventLogger {
	return &EventLogger{
		dir:   filepath.Join(workspace, "logs"),
		runID: runID,
	}
}

// Emit writes a structured event to logs/<component>.jsonl. When
// sessionID is non-empty, it is stored at the top level (parallel to
// run_id) for easy filtering of per-session streams.
func (l *EventLogger) Emit(component, event, severity, sessionID string, payload map[string]any) {
	if l == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	record := map[string]any{
		"ts":        time.Now().UTC().Format(time.RFC3339Nano),
		"run_id":    l.runID,
		"component": component,
		"event":     event,
		"severity":  severity,
	}
	if sessionID != "" {
		record["session_id"] = sessionID
	}
	record["payload"] = payload

	raw, err := json.Marshal(record)
	if err != nil {
		log.Printf("[eventlog] marshal error for %s/%s: %v", component, event, err)
		return
	}

	file := component
	switch component {
	case "retrieval", "runtime", "session":
		// allowed
	default:
		file = "runtime"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_ = os.MkdirAll(l.dir, 0755)
	fp := filepath.Join(l.dir, file+".jsonl")
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(string(raw) + "\n")
}

// NewRunID returns a fresh, file-system-safe, timestamp-ordered run identifier
// of the form "run-YYYYMMDD-HHMMSS-<8-hex>". The timestamp is UTC so multiple
// hosts (or docker containers with skewed local time) still produce
// lexicographically-ordered IDs when collated.
//
// The suffix is 8 hex characters drawn from crypto/rand (~4.3B space). With
// the birthday paradox, even ~100k calls within the same second carry a
// collision probability below 10^-6 — effectively nil for agent workloads
// that produce at most a few runs per second.
//
// Example: "run-20260419-153027-8f3a2b17".
func NewRunID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("run-%s-%08d",
			time.Now().UTC().Format("20060102-150405"),
			time.Now().UnixNano()%100000000,
		)
	}
	return fmt.Sprintf("run-%s-%s",
		time.Now().UTC().Format("20060102-150405"),
		hex.EncodeToString(buf),
	)
}

// SanitizeRunIDForFilename strips any character that is not safe for use as
// a file-system path component from id, replacing runs of unsafe characters
// with a single '-'. The result is guaranteed to match [A-Za-z0-9._-]+ and
// to be non-empty: if all characters of id are stripped the function
// returns "run-unknown" so downstream callers never accidentally write to
// the parent directory itself.
//
// NewRunID() already returns a safe string, so this function is effectively
// the identity transform on current inputs. It is exported as a contract:
// any code path that concatenates a run_id into a path MUST go through it,
// so that future format changes to NewRunID cannot silently introduce
// directory traversal or other path-injection hazards.
func SanitizeRunIDForFilename(id string) string {
	if id == "" {
		return "run-unknown"
	}
	var b strings.Builder
	b.Grow(len(id))
	lastDash := false
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "run-unknown"
	}
	return out
}
