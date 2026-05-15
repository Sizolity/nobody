package llamacpp

// Integration coverage for the Manager reuse path. Pairs with
// manager_test.go (state-machine units, reuse via injected forkFn,
// Close idempotence) and the env-gated E2E in
// e2e_llamacpp_managed_test.go (real `llama-server` fork). What this
// file uniquely covers is the network-level reuse contract: an
// httptest.Server actually binds the configured port and answers
// `/health` on a real socket, so we exercise the production
// `m.healthOK(ctx)` HTTP path rather than an in-memory stub.
//
// The crash-after-fork path is intentionally not covered here.
// Constructing a deterministic "fork succeeds → /health goes OK →
// process dies" sequence in pure Go (without spawning a real
// llama-server) requires racing httptest binding against a fork-fn
// substitute that sleeps in the right window — fragile, and the
// behaviour we'd be locking is already covered by T6's real-binary
// E2E. See the plan, Task 5 § "ForkPathThenCrash deferred".

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sizolity/nobody/internal/config"
)

// TestManagerIntegration_FakeServer_FullReuseFlow exercises the
// happy-path reuse cycle end-to-end:
//
//  1. An httptest.Server occupies the chosen port and answers /health 200.
//  2. Manager.Start probes once via the production HTTP client, sees a
//     healthy server, takes the reuse=true branch (no fork).
//  3. Manager.IsReused returns true.
//  4. Manager.Close is a no-op for reused servers; the external server
//     stays alive afterwards.
//  5. Exactly one inference_managed_reuse event is emitted; no crash
//     event fires (we did not own the process, so its eventual shutdown
//     does not look like a crash to us).
func TestManagerIntegration_FakeServer_FullReuseFlow(t *testing.T) {
	t.Parallel()

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
		t.Fatalf("parse httptest URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	var (
		mu       sync.Mutex
		captured []string
	)
	emit := func(name, _ string, _ map[string]any) {
		mu.Lock()
		captured = append(captured, name)
		mu.Unlock()
	}

	mc := config.ManagedConfig{
		Port:          port,
		Host:          "127.0.0.1",
		HealthTimeout: 5 * time.Second,
	}
	mgr, err := NewManager(mc, emit)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !mgr.IsReused() {
		t.Fatalf("IsReused=false, want true (httptest.Server was bound to the configured port)")
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	var reuseSeen, crashSeen, startSeen int
	for _, name := range captured {
		switch name {
		case "inference_managed_reuse":
			reuseSeen++
		case "inference_managed_start":
			startSeen++
		case "inference_managed_crash":
			crashSeen++
		}
	}
	if reuseSeen != 1 {
		t.Errorf("inference_managed_reuse fired %d times, want exactly 1; events=%v", reuseSeen, captured)
	}
	if startSeen != 0 {
		t.Errorf("inference_managed_start unexpectedly fired %d times on reuse path; events=%v", startSeen, captured)
	}
	if crashSeen != 0 {
		t.Errorf("inference_managed_crash unexpectedly fired %d times on reuse path; events=%v", crashSeen, captured)
	}

	// Reused server must remain alive after Close — we did not own it,
	// so Close is a documented no-op (manager.go IsReused branch).
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("post-Close /health probe failed (server should still be up): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("post-Close /health=%d, want 200 (Close must NOT have killed the external server)", resp.StatusCode)
	}
}
