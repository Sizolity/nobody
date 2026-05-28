package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sizolity/nobody/internal/world/ingest"
	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/role"
)

func TestBuildCombo(t *testing.T) {
	choices := role.ActionChoices{Options: []role.ActionOption{
		{Label: "A", Type: role.ActionTypeExplore},
		{Label: "B", Type: role.ActionTypeSocial},
		{Label: "C", Type: role.ActionTypeInvestigate},
		{Type: role.ActionTypeCustom},
	}}

	tests := []struct {
		name string
		line string
		want string
		ok   bool
	}{
		{"two-digit combo", "32", "【组合行动】先 C，最后 B", true},
		{"three-digit combo", "213", "【组合行动】先 B，再 A，最后 C", true},
		{"single digit goes through buildCombo too", "1", "【组合行动】先 A", true},
		{"out of range", "39", "", false},
		{"contains custom slot", "14", "", false},
		{"all custom slot", "44", "", false},
		{"zero is invalid", "10", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := buildCombo(tt.line, choices)
			if ok != tt.ok {
				t.Fatalf("ok=%v want=%v (got=%q)", ok, tt.ok, got)
			}
			if ok && got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"1", true},
		{"32", true},
		{"123", true},
		{"1a", false},
		{"a1", false},
		{" 1", false},
	}
	for _, tt := range tests {
		if got := isAllDigits(tt.in); got != tt.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// === Sub 5: Lorekeeper status footer + cmdStatus world-knowledge ===

func TestSummarizeLoreReport_Empty(t *testing.T) {
	var r ingest.CompileReport
	if got := summarizeLoreReport(r); got != "" {
		t.Fatalf("expected empty string for zero report, got %q", got)
	}
}

func TestSummarizeLoreReport_Basic(t *testing.T) {
	r := ingest.CompileReport{Inserted: 2, Skipped: 1}
	want := "沉淀: 插入=2 跳过=1 拒绝=0 过滤=0"
	if got := summarizeLoreReport(r); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestSummarizeLoreReport_WithNotes(t *testing.T) {
	r := ingest.CompileReport{
		Inserted: 1,
		Notes:    []string{"validate-warn: foo", "compile-reject: bar"},
	}
	want := "沉淀: 插入=1 跳过=0 拒绝=0 过滤=0 (含 2 条提示)"
	if got := summarizeLoreReport(r); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestTopNPCsByMemoryCount_Empty(t *testing.T) {
	got := topNPCsByMemoryCount(worldmodel.World{}, 5)
	if len(got) != 0 {
		t.Fatalf("expected empty slice for empty world, got %v", got)
	}
}

func TestTopNPCsByMemoryCount_OrderAndCap(t *testing.T) {
	// Seven NPCs with memory counts {3, 1, 5, 5, 2, 4, 0}.
	// The zero-count NPC must be excluded; the rest sorted by count desc
	// then ID asc; truncated to top 5.
	specs := []struct {
		id    string
		count int
	}{
		{"npc-a", 3},
		{"npc-b", 1},
		{"npc-c", 5},
		{"npc-d", 5},
		{"npc-e", 2},
		{"npc-f", 4},
		{"npc-g", 0},
	}
	entities := map[worldmodel.EntityID]worldmodel.Entity{}
	var mem []worldmodel.MemoryRecord
	for _, s := range specs {
		eid := worldmodel.EntityID(s.id)
		entities[eid] = worldmodel.Entity{
			ID:   eid,
			Type: "character",
			Name: "name-" + s.id,
		}
		for i := 0; i < s.count; i++ {
			mem = append(mem, worldmodel.MemoryRecord{
				Owner: worldmodel.MemoryOwner{
					Kind: worldmodel.MemoryOwnerKindCharacter,
					ID:   s.id,
				},
			})
		}
	}
	world := worldmodel.World{Entities: entities, Memory: mem}

	got := topNPCsByMemoryCount(world, 5)
	if len(got) != 5 {
		t.Fatalf("expected top 5, got %d (%+v)", len(got), got)
	}

	wantOrder := []struct {
		id    string
		count int
	}{
		{"npc-c", 5},
		{"npc-d", 5},
		{"npc-f", 4},
		{"npc-a", 3},
		{"npc-e", 2},
	}
	for i, want := range wantOrder {
		if string(got[i].ID) != want.id || got[i].Count != want.count {
			t.Errorf("rank %d: want id=%s count=%d, got id=%s count=%d",
				i, want.id, want.count, got[i].ID, got[i].Count)
		}
	}

	for _, s := range got {
		if string(s.ID) == "npc-g" {
			t.Errorf("zero-count NPC npc-g must not be included")
		}
		if string(s.ID) == "npc-b" {
			t.Errorf("npc-b (count=1) is outside top 5 but appeared")
		}
	}
}

func TestTopNPCsByMemoryCount_OnlyCharacterOwned(t *testing.T) {
	// Memories with non-character owners must NOT contribute to any NPC
	// counter. Only character-owned memories count.
	mem := []worldmodel.MemoryRecord{
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindWorld}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindWorld}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindNarrator, ID: "narr-1"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindFaction, ID: "fac-1"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindCharacter, ID: "npc-x"}},
		{Owner: worldmodel.MemoryOwner{Kind: worldmodel.MemoryOwnerKindCharacter, ID: "npc-x"}},
	}
	entities := map[worldmodel.EntityID]worldmodel.Entity{
		"npc-x": {ID: "npc-x", Type: "character", Name: "X"},
	}
	world := worldmodel.World{Entities: entities, Memory: mem}

	got := topNPCsByMemoryCount(world, 5)
	if len(got) != 1 {
		t.Fatalf("expected exactly one NPC (only character owner), got %d (%+v)", len(got), got)
	}
	if got[0].Count != 2 {
		t.Errorf("expected count=2 (two character-kind memories), got %d", got[0].Count)
	}
	if string(got[0].ID) != "npc-x" {
		t.Errorf("expected ID=npc-x, got %s", got[0].ID)
	}
	if got[0].Name != "X" {
		t.Errorf("expected Name=X, got %s", got[0].Name)
	}
}

// TestIsRetryableHTTPErr pins which transport-layer errors are safe to
// replay. Only PRE-response errors (connection failed before any
// response bytes) are retryable; user-cancellation and post-response
// failures are not.
func TestIsRetryableHTTPErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"url.Error wrapping io.EOF (DeepSeek pattern)", &url.Error{Op: "Post", URL: "https://example/v1/chat", Err: io.EOF}, true},
		{"connection reset", errors.New("read tcp 1.2.3.4:443: connection reset by peer"), true},
		{"connection refused", errors.New("dial tcp 1.2.3.4:443: connection refused"), true},
		{"broken pipe", errors.New("write tcp: broken pipe"), true},
		{"TLS handshake EOF", errors.New("tls: handshake EOF"), true},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"plain 4xx-shaped error", errors.New("status 400 bad request"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableHTTPErr(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableHTTPErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestRetryingTransport_RetriesOnEOF simulates DeepSeek's "server closes
// the TCP connection before responding" pattern using a httptest server
// that hijacks and closes its first two connections, then succeeds on
// the third. The transport must retry transparently and surface the
// final 200 to the caller.
func TestRetryingTransport_RetriesOnEOF(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := newRetryingHTTPClient(5*time.Second, httpRetryConfig{
		MaxAttempts: 5,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     5 * time.Millisecond,
	})

	resp, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("server saw %d attempts, want exactly 3 (2 EOFs + 1 success)", got)
	}
}

// TestRetryingTransport_GivesUpAfterMaxAttempts ensures the retry
// budget is bounded — an endlessly-failing server eventually surfaces
// the last error to the caller instead of retrying forever.
func TestRetryingTransport_GivesUpAfterMaxAttempts(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer server.Close()

	client := newRetryingHTTPClient(2*time.Second, httpRetryConfig{
		MaxAttempts: 3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     2 * time.Millisecond,
	})

	_, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("server saw %d attempts, want exactly 3 (max budget)", got)
	}
}

// TestRetryingTransport_DoesNotRetryHTTP4xx confirms that a real HTTP
// error response (4xx) is NOT a transport-layer failure — RoundTrip
// returns a valid *Response and nil error, so retry must not engage.
func TestRetryingTransport_DoesNotRetryHTTP4xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newRetryingHTTPClient(2*time.Second, defaultRetryConfig())

	resp, err := client.Post(server.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d want 400", resp.StatusCode)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("server saw %d attempts, want 1 (4xx is not transport-retryable)", got)
	}
}
