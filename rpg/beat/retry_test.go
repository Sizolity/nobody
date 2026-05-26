package beat

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
)

func TestRepairJSON_AlreadyValid(t *testing.T) {
	input := `{"name":"Alice","level":3}`
	got := RepairJSON(input)
	if got != input {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestRepairJSON_MarkdownFences(t *testing.T) {
	input := "```json\n{\"a\":1}\n```"
	got := RepairJSON(input)
	want := `{"a":1}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_MarkdownFencesNoLang(t *testing.T) {
	input := "```\n{\"a\":1}\n```"
	got := RepairJSON(input)
	want := `{"a":1}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_TrailingCommaObject(t *testing.T) {
	input := `{"a":1, "b":2, }`
	got := RepairJSON(input)
	want := `{"a":1, "b":2}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_TrailingCommaArray(t *testing.T) {
	input := `[1, 2, 3, ]`
	got := RepairJSON(input)
	want := `[1, 2, 3]`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_TextBeforeJSON(t *testing.T) {
	input := `Sure, here is the JSON: {"key":"val"}`
	got := RepairJSON(input)
	want := `{"key":"val"}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_TextAfterJSON(t *testing.T) {
	input := `{"key":"val"} Hope that helps!`
	got := RepairJSON(input)
	want := `{"key":"val"}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRepairJSON_RawNewlines(t *testing.T) {
	input := "{\"text\":\"line one\nline two\"}"
	got := RepairJSON(input)
	want := `{"text":"line one\nline two"}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("repaired JSON still invalid: %v", err)
	}
}

func TestRepairJSON_Combined(t *testing.T) {
	input := "Here you go:\n```json\n{\"items\": [1, 2, ],}\n```\nEnjoy!"
	got := RepairJSON(input)
	want := `{"items": [1, 2]}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- RetryWithRepair tests ---

func parseMap(s string) (map[string]any, error) {
	var m map[string]any
	err := json.Unmarshal([]byte(s), &m)
	return m, err
}

func TestRetryWithRepair_FirstAttemptSucceeds(t *testing.T) {
	ctx := context.Background()
	calls := 0
	result, err := RetryWithRepair(ctx, 3,
		func(_ context.Context) (string, error) {
			calls++
			return `{"ok":true}`, nil
		},
		parseMap,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected result: %v", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithRepair_RepairSucceeds(t *testing.T) {
	ctx := context.Background()
	calls := 0
	result, err := RetryWithRepair(ctx, 3,
		func(_ context.Context) (string, error) {
			calls++
			return "```json\n{\"ok\":true,}\n```", nil
		},
		parseMap,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected result: %v", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (repair, no retry), got %d", calls)
	}
}

func TestRetryWithRepair_AllFail(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int32
	_, err := RetryWithRepair(ctx, 2,
		func(_ context.Context) (string, error) {
			calls.Add(1)
			return "not json at all !!!", nil
		},
		parseMap,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestRetryWithRepair_SecondAttemptSucceeds(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int32
	result, err := RetryWithRepair(ctx, 3,
		func(_ context.Context) (string, error) {
			n := calls.Add(1)
			if n == 1 {
				return "", errors.New("network blip")
			}
			return `{"retry":"worked"}`, nil
		},
		parseMap,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["retry"] != "worked" {
		t.Fatalf("unexpected result: %v", result)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestRetryWithRepair_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RetryWithRepair(ctx, 5,
		func(_ context.Context) (string, error) {
			return `{"a":1}`, nil
		},
		parseMap,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
