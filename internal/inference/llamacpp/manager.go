package llamacpp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/sizolity/nobody/internal/config"
)

// EmitFn is the minimal sink interface the Manager needs to surface
// lifecycle events. Matches the signature of harness.EventLogger.Emit
// curried with component="runtime" and sessionID="" so callers do not
// have to import that type.
type EmitFn func(eventName, severity string, payload map[string]any)

// managerState is the explicit state machine entry. See spec §2.3 for
// the transition diagram. All transitions hold m.mu.
type managerState int

const (
	stateIdle     managerState = iota // freshly constructed; Start not yet called
	stateStarting                     // Start in progress: probing or forking
	stateRunning                      // /health 200 reached; chat traffic OK
	stateDead                         // managed-fork exited unexpectedly while running
	stateClosed                       // Close completed; terminal
)

// managerLastLogLines caps the size of the last_log_lines payload field
// on inference_managed_crash events. Const-only; not yaml-exposed.
const managerLastLogLines = 20

// Manager owns the lifecycle of an external llama-server chat process
// when nobody.yaml has provider_opts.llamacpp.lifecycle=managed.
//
// Manager is constructed via NewManager and held by harness.Harness; it
// does NOT implement any interface in inference/ — process supervision stays
// private to the llama.cpp runtime.
type Manager struct {
	cfg     config.ManagedConfig
	rawEmit EmitFn
	kind    string

	// forkFn injects the subprocess starter so tests can substitute a
	// fake. Production = exec.Cmd.Start. Returns the *os.Process so
	// Close can SIGTERM it; the wait func runs in a goroutine to feed
	// crash detection.
	forkFn func(ctx context.Context, cfg config.ManagedConfig) (*os.Process, func() error, error)

	// httpClient is the probe client; tests override to add custom
	// transports if needed. Default = http.Client with 2s timeout per
	// request (the loop adds outer timeout via cfg.HealthTimeout).
	httpClient *http.Client

	mu      sync.Mutex
	state   managerState
	reused  bool
	proc    *os.Process   // nil when reused or never forked
	closing bool          // true between Close start and finish; suppresses crash event
	lastLog []string      // ring buffer of stdout/stderr lines for crash payload
	logMu   sync.Mutex    // guards lastLog
	done    chan struct{} // closed when watchExit returns; nil if no watcher launched
}

// NewManager validates the structural minima of cfg and returns a
// Manager in stateIdle. Network and fork calls happen in Start.
func NewManager(cfg config.ManagedConfig, emit EmitFn) (*Manager, error) {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("managed.port=%d out of range", cfg.Port)
	}
	if cfg.HealthTimeout <= 0 {
		cfg.HealthTimeout = 30 * time.Second
	}
	if emit == nil {
		emit = func(string, string, map[string]any) {}
	}
	m := &Manager{
		cfg:        cfg,
		rawEmit:    emit,
		kind:       "chat",
		forkFn:     defaultForkFn,
		httpClient: &http.Client{Timeout: 2 * time.Second},
		state:      stateIdle,
	}
	return m, nil
}

func NewManagerWithKind(kind string, cfg config.ManagedConfig, emit EmitFn) (*Manager, error) {
	m, err := NewManager(cfg, emit)
	if err != nil {
		return nil, err
	}
	if kind != "" {
		m.kind = kind
	}
	return m, nil
}

// emit wraps rawEmit with provider stamping (spec §5 trial of payload
// auto-stamping; remains llamacpp-internal until Inference Layer
// deferred item #5 is generalised).
func (m *Manager) emit(eventName, severity string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["provider"] = ProviderName
	payload["kind"] = m.kind
	m.rawEmit(eventName, severity, payload)
}

// IsReused reports whether the most recent successful Start detected a
// pre-existing healthy server on the configured port. False until
// Start succeeds; remains false across re-fork after a crash (the
// re-forked process is owned by us).
func (m *Manager) IsReused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reused
}

// Start probes (host:port)/health; if 200, marks reused=true and returns
// without forking. Otherwise forks llama-server with cfg-derived argv
// and waits for /health 200 within cfg.HealthTimeout. Subsequent Start
// calls after a recorded crash trigger re-fork (state = dead → starting).
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	switch m.state {
	case stateIdle, stateDead:
		// fallthrough: proceed with start
	case stateStarting:
		m.mu.Unlock()
		return errors.New("manager Start called concurrently")
	case stateRunning:
		m.mu.Unlock()
		return nil
	case stateClosed:
		m.mu.Unlock()
		return errors.New("manager Start after Close")
	}
	m.state = stateStarting
	m.reused = false
	m.proc = nil
	m.mu.Unlock()

	if m.healthOK(ctx) {
		m.mu.Lock()
		m.state = stateRunning
		m.reused = true
		m.mu.Unlock()
		m.emit("inference_managed_reuse", "info", map[string]any{
			"port":   m.cfg.Port,
			"source": "external",
		})
		return nil
	}

	startedAt := time.Now()
	proc, waitFn, err := m.forkFn(ctx, m.cfg)
	if err != nil {
		m.mu.Lock()
		m.state = stateDead
		m.mu.Unlock()
		return fmt.Errorf("fork llama-server: %w", err)
	}
	m.mu.Lock()
	m.proc = proc
	m.mu.Unlock()

	deadline := time.Now().Add(m.cfg.HealthTimeout)
	for {
		if ctx.Err() != nil {
			_ = m.killProcess(proc)
			m.mu.Lock()
			m.state = stateDead
			m.mu.Unlock()
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			_ = m.killProcess(proc)
			m.mu.Lock()
			m.state = stateDead
			m.mu.Unlock()
			return fmt.Errorf("llama-server did not become healthy within %s", m.cfg.HealthTimeout)
		}
		if m.healthOK(ctx) {
			break
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
		}
	}

	m.mu.Lock()
	m.state = stateRunning
	m.done = make(chan struct{})
	m.mu.Unlock()
	m.emit("inference_managed_start", "info", map[string]any{
		"port":             m.cfg.Port,
		"pid":              proc.Pid,
		"model":            m.cfg.Model,
		"ngl":              derefIntOrZero(m.cfg.NGL),
		"ctx":              derefIntOrZero(m.cfg.Ctx),
		"fork_duration_ms": time.Since(startedAt).Milliseconds(),
	})

	go m.watchExit(waitFn, proc.Pid)

	return nil
}

// Close stops the managed process iff IsReused() is false. SIGTERM,
// then SIGKILL after 10s. Idempotent. nop on reused servers (we did
// not fork them).
//
// Concurrency: Close MUST NOT be called concurrently with Start.
// Callers (e.g. harness.New + harness.Close) are expected to await
// Start completion before invoking Close. Concurrent Start+Close is
// undefined behaviour in v1; a future revision may serialize this
// internally.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.state == stateClosed {
		m.mu.Unlock()
		return nil
	}
	m.closing = true
	reused := m.reused
	proc := m.proc
	done := m.done
	m.state = stateClosed
	m.mu.Unlock()

	if reused || proc == nil {
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		_ = proc.Kill()
	} else {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if !processAlive(proc.Pid) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if processAlive(proc.Pid) {
			_ = proc.Kill()
		}
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
		}
	}
	return nil
}

// healthOK does a single /health probe; returns true on 200. Start's
// loop owns retry timing so this is intentionally one-shot.
func (m *Manager) healthOK(ctx context.Context) bool {
	url := fmt.Sprintf("http://%s:%d/health", m.cfg.Host, m.cfg.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// killProcess best-effort; ignores errors when the process is already
// gone. Used for fork-then-health-fails cleanup.
func (m *Manager) killProcess(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	if processAlive(proc.Pid) {
		return proc.Kill()
	}
	return nil
}

// watchExit blocks on cmd.Wait via waitFn, then emits
// inference_managed_crash unless the manager is shutting down (Close
// in progress = expected exit, no event needed).
func (m *Manager) watchExit(waitFn func() error, pid int) {
	m.mu.Lock()
	done := m.done
	m.mu.Unlock()
	if done != nil {
		defer close(done)
	}
	exitErr := waitFn()
	m.mu.Lock()
	closing := m.closing
	if m.state != stateClosed {
		m.state = stateDead
	}
	m.proc = nil
	m.mu.Unlock()
	if closing {
		return
	}
	exitCode := -1
	if exitErr != nil {
		var ee *exec.ExitError
		if errors.As(exitErr, &ee) {
			exitCode = ee.ExitCode()
		}
	} else {
		exitCode = 0
	}
	m.logMu.Lock()
	tail := append([]string(nil), m.lastLog...)
	m.logMu.Unlock()
	if len(tail) > managerLastLogLines {
		tail = tail[len(tail)-managerLastLogLines:]
	}
	m.emit("inference_managed_crash", "error", map[string]any{
		"port":           m.cfg.Port,
		"pid":            pid,
		"exit_code":      exitCode,
		"last_log_lines": tail,
	})
}

// defaultForkFn assembles the llama-server argv and starts the
// subprocess. Returns the process handle and a wait function the
// caller's crash watcher will invoke. Output is currently discarded
// — wiring stdout/stderr into m.lastLog requires Manager fields not
// visible to the closure; tracked as a follow-up (spec §3.1 share-dir
// log is best-effort, not v1-required).
func defaultForkFn(ctx context.Context, cfg config.ManagedConfig) (*os.Process, func() error, error) {
	args := buildForkArgs(cfg)
	cmd := exec.CommandContext(ctx, cfg.Bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd.Process, cmd.Wait, nil
}

func buildForkArgs(cfg config.ManagedConfig) []string {
	args := []string{
		"--model", cfg.Model,
		"--host", cfg.Host,
		"--port", fmt.Sprintf("%d", cfg.Port),
		"-c", fmt.Sprintf("%d", derefIntOrZero(cfg.Ctx)),
	}
	if v := derefIntOrZero(cfg.NGL); v > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", v))
	}
	if cfg.Template != "" {
		args = append(args, "--chat-template", cfg.Template)
	}
	args = append(args, cfg.ExtraFlags...)
	return args
}

func derefIntOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// processAlive checks whether a Unix process with the given PID is
// alive. Uses signal 0 which is the canonical "is alive" probe on
// POSIX. Returns false on Windows where this approach is incorrect —
// production target is Linux/macOS per the rest of nobody.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscallSignalZero); err != nil {
		return false
	}
	return true
}
