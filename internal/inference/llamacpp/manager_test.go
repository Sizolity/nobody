package llamacpp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

type recordingEmit struct {
	events []emittedEvent
}

type emittedEvent struct {
	name     string
	severity string
	payload  map[string]any
}

func (r *recordingEmit) emit(name, severity string, payload map[string]any) {
	r.events = append(r.events, emittedEvent{name: name, severity: severity, payload: payload})
}

func TestManager_StartReusesExistingHealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse fake server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	rec := &recordingEmit{}
	mc := config.ManagedConfig{
		Bin:           "should-not-be-invoked",
		Model:         "/dev/null",
		Port:          port,
		Host:          "127.0.0.1",
		HealthTimeout: 5 * time.Second,
	}
	mgr, err := NewManager(mc, rec.emit)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !mgr.IsReused() {
		t.Errorf("IsReused = false, want true")
	}
	if err := mgr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("post-Close /health probe failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("post-Close /health status = %d, want 200", resp.StatusCode)
	}
	var reuseCount int
	for _, e := range rec.events {
		if e.name == "inference_managed_reuse" {
			reuseCount++
			if e.payload["port"] != port {
				t.Errorf("reuse payload port = %v, want %d", e.payload["port"], port)
			}
		}
	}
	if reuseCount != 1 {
		t.Errorf("inference_managed_reuse fired %d times, want 1", reuseCount)
	}
}

func TestManager_ReusePayloadIncludesKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	rec := &recordingEmit{}
	mc := config.ManagedConfig{Port: port, Host: "127.0.0.1", HealthTimeout: time.Second}
	mgr, err := NewManagerWithKind("embedding", mc, rec.emit)
	require.NoError(t, err)
	require.NoError(t, mgr.Start(context.Background()))

	require.Len(t, rec.events, 1)
	require.Equal(t, "inference_managed_reuse", rec.events[0].name)
	require.Equal(t, "embedding", rec.events[0].payload["kind"])
	require.Equal(t, "llamacpp", rec.events[0].payload["provider"])
}

func TestDefaultForkFn_EmbeddingArgsIncludeEmbeddingFlag(t *testing.T) {
	got := buildForkArgs(config.ManagedConfig{
		Bin:        "llama-server",
		Model:      "/models/embed.gguf",
		Host:       "127.0.0.1",
		Port:       18081,
		ExtraFlags: []string{"--log-disable", "--embedding"},
	})
	require.Contains(t, got, "--embedding")
	require.Contains(t, got, "--model")
	require.Contains(t, got, "/models/embed.gguf")
}

func TestManager_StartFailsWhenForkErrors(t *testing.T) {
	rec := &recordingEmit{}
	mc := config.ManagedConfig{
		Bin:           "/nonexistent/llama-server",
		Model:         "/dev/null",
		Port:          1, // port 1 is privileged, will not be healthy in test env
		Host:          "127.0.0.1",
		HealthTimeout: 1 * time.Second,
	}
	mgr, err := NewManager(mc, rec.emit)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.forkFn = func(ctx context.Context, cfg config.ManagedConfig) (*os.Process, func() error, error) {
		return nil, nil, errors.New("simulated fork failure")
	}
	if err := mgr.Start(context.Background()); err == nil {
		t.Fatalf("expected fork error, got nil")
	}
	if mgr.IsReused() {
		t.Errorf("IsReused=true, want false")
	}
}

// TestManager_DoubleClose verifies idempotent Close.
func TestManager_DoubleClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	mc := config.ManagedConfig{
		Bin: "should-not-be-invoked", Model: "/dev/null",
		Port: port, Host: "127.0.0.1",
		HealthTimeout: 5 * time.Second,
	}
	mgr, _ := NewManager(mc, func(string, string, map[string]any) {})
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Errorf("second Close: %v (must be nop)", err)
	}
}

// TestManager_StartAfterCloseFails verifies the closed terminal state.
func TestManager_StartAfterCloseFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	mc := config.ManagedConfig{
		Port: port, Host: "127.0.0.1",
		HealthTimeout: 5 * time.Second,
	}
	mgr, _ := NewManager(mc, func(string, string, map[string]any) {})
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := mgr.Start(context.Background()); err == nil {
		t.Errorf("Start after Close = nil, want error")
	}
}

// TestManager_NewManagerRejectsBadPort locks the input validation in NewManager.
func TestManager_NewManagerRejectsBadPort(t *testing.T) {
	for _, port := range []int{0, -1, 65536, 70000} {
		mc := config.ManagedConfig{Port: port, Host: "127.0.0.1", HealthTimeout: 5 * time.Second}
		if _, err := NewManager(mc, nil); err == nil {
			t.Errorf("NewManager(port=%d) returned nil error", port)
		}
	}
}

// TestManager_StateClosedNotClobberedByWatchExit locks T3 review C1:
// when Close() runs while watchExit is mid-Wait, the goroutine must
// not write state=stateDead over state=stateClosed. Regression: the
// original watchExit unconditionally wrote stateDead, allowing a
// subsequent Start() to re-fork on a "closed" manager.
//
// We construct internal state directly (same-package access) rather
// than going through Start, so we can isolate the watchExit-vs-Close
// race without staging a full /health probe path.
func TestManager_StateClosedNotClobberedByWatchExit(t *testing.T) {
	mc := config.ManagedConfig{
		Bin: "fake", Model: "/dev/null",
		Port: 8080, Host: "127.0.0.1",
		HealthTimeout: 5 * time.Second,
	}
	mgr, _ := NewManager(mc, func(string, string, map[string]any) {})

	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	mgr.mu.Lock()
	mgr.state = stateRunning
	mgr.proc = cmd.Process
	mgr.done = make(chan struct{})
	mgr.mu.Unlock()
	go mgr.watchExit(cmd.Wait, cmd.Process.Pid)

	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Assert the specific "Start after Close" error rather than any error:
	// without C1, watchExit would clobber state=stateClosed → state=stateDead,
	// and Start on stateDead enters the fork path which would fail on Bin="fake"
	// for an unrelated reason (still err != nil but wrong path). Checking the
	// exact error text ensures the test catches the regression deterministically.
	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatalf("Start after Close succeeded — watchExit clobbered stateClosed (C1 regression)")
	}
	if !strings.Contains(err.Error(), "after Close") {
		t.Errorf("Start after Close returned unexpected error %q; want text containing \"after Close\" (state likely clobbered to stateDead, hit fork path instead)", err.Error())
	}
}
