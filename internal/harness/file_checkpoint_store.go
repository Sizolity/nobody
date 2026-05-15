package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// stateEnvelope is the single JSON document persisted for a task.
// It carries both the Eino checkpoint bytes (keyed by checkpoint ID)
// and the latest orchestrator snapshot, plus process metadata used by
// crash recovery (D22-D25).
type stateEnvelope struct {
	SchemaVersion   int               `json:"schema_version"`
	TaskHash        string            `json:"task_hash"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Pid             int               `json:"pid"`
	Hostname        string            `json:"hostname"`
	StartedAt       time.Time         `json:"started_at"`
	LastHeartbeatAt time.Time         `json:"last_heartbeat_at"`
	Checkpoints     map[string]string `json:"checkpoints"` // id -> hex(bytes)
	LatestSnapshot  json.RawMessage   `json:"latest_snapshot,omitempty"`
}

const stateEnvelopeSchemaVersion = 1

// FileCheckPointStore persists Eino checkpoints + orchestrator snapshots
// to a single JSON file per task, using atomic rename for durability.
// Implements compose.CheckPointStore via Get/Set; the legacy
// SaveSnapshot/LoadSnapshot interface used by harness.go's ad-hoc
// recovery path is also preserved.
type FileCheckPointStore struct {
	mu       sync.Mutex
	dir      string
	taskHash string
	envelope stateEnvelope
}

// NewFileCheckPointStore creates (or loads) the state file for task.
// Returns the store + whether a previous envelope existed (for crash
// recovery decision making).
func NewFileCheckPointStore(stateDir, task string) (*FileCheckPointStore, bool, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, false, fmt.Errorf("mkdir state dir: %w", err)
	}
	h := sha256.Sum256([]byte(task))
	hash := hex.EncodeToString(h[:16]) // 32 hex chars

	s := &FileCheckPointStore{
		dir:      stateDir,
		taskHash: hash,
		envelope: stateEnvelope{
			SchemaVersion: stateEnvelopeSchemaVersion,
			TaskHash:      hash,
			Checkpoints:   map[string]string{},
		},
	}
	existed, err := s.load()
	if err != nil {
		return nil, false, err
	}
	return s, existed, nil
}

func (s *FileCheckPointStore) path() string {
	return filepath.Join(s.dir, s.taskHash+".json")
}

func (s *FileCheckPointStore) load() (bool, error) {
	data, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read state file: %w", err)
	}
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return false, fmt.Errorf("unmarshal state file: %w", err)
	}
	if env.Checkpoints == nil {
		env.Checkpoints = map[string]string{}
	}
	s.envelope = env
	return true, nil
}

// flush writes envelope to disk atomically (write-tmp + rename).
// Caller must hold s.mu.
func (s *FileCheckPointStore) flush() error {
	s.envelope.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, s.path())
}

// Envelope returns a snapshot of the envelope metadata (for crash
// recovery flow to inspect phase etc. before any Get is called).
func (s *FileCheckPointStore) Envelope() stateEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envelope
}

// Get / Set implement compose.CheckPointStore. Keys are Eino checkpoint
// IDs; values are opaque bytes. We store them as hex nested inside the
// JSON envelope so the file is atomic per-write.
func (s *FileCheckPointStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.envelope.Checkpoints[id]
	if !ok {
		return nil, false, nil
	}
	data, err := hex.DecodeString(raw)
	if err != nil {
		return nil, false, fmt.Errorf("decode checkpoint %s: %w", id, err)
	}
	return data, true, nil
}

func (s *FileCheckPointStore) Set(_ context.Context, id string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envelope.Checkpoints[id] = hex.EncodeToString(data)
	return s.flush()
}

// SaveSnapshot stores the latest orchestrator snapshot bytes (TaskState
// or result envelope). Matches the existing InMemoryCheckPointStore API
// used by harness.go.
func (s *FileCheckPointStore) SaveSnapshot(taskID string, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envelope.LatestSnapshot = append(json.RawMessage(nil), payload...)
	_ = s.flush() // best-effort; caller doesn't check err today
}

func (s *FileCheckPointStore) LoadSnapshot(taskID string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.envelope.LatestSnapshot) == 0 {
		return nil, false
	}
	out := make([]byte, len(s.envelope.LatestSnapshot))
	copy(out, s.envelope.LatestSnapshot)
	return out, true
}

// Clear wipes the Eino checkpoint entries but preserves the latest
// orchestrator snapshot and process metadata. Matches the semantics of
// InMemoryCheckPointStore.Clear (keep snapshot:* keys, drop the rest).
func (s *FileCheckPointStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envelope.Checkpoints = map[string]string{}
	_ = s.flush()
}

// UpdateMetadata merges process/heartbeat fields and flushes.
func (s *FileCheckPointStore) UpdateMetadata(fn func(e *stateEnvelope)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.envelope)
	return s.flush()
}
