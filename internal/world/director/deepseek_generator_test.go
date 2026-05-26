package director

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeepSeekGeneratorSendsCorrectRequest(t *testing.T) {
	t.Parallel()

	var gotReq chatRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Role: "assistant", Content: "[]"}}},
		})
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{
		APIKey:     "sk-test-key",
		Model:      "deepseek-v4-pro",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	_, err := g.Generate(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if gotAuth != "Bearer sk-test-key" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotReq.Model != "deepseek-v4-pro" {
		t.Fatalf("model = %q", gotReq.Model)
	}
	if len(gotReq.Messages) != 2 {
		t.Fatalf("messages count = %d, want 2", len(gotReq.Messages))
	}
	if gotReq.Messages[0].Role != "system" || gotReq.Messages[0].Content != "system prompt" {
		t.Fatalf("system message = %+v", gotReq.Messages[0])
	}
	if gotReq.Messages[1].Role != "user" || gotReq.Messages[1].Content != "user prompt" {
		t.Fatalf("user message = %+v", gotReq.Messages[1])
	}
	if gotReq.Stream {
		t.Fatal("stream should be false")
	}
}

func TestDeepSeekGeneratorOmitsSystemWhenEmpty(t *testing.T) {
	t.Parallel()

	var gotReq chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Role: "assistant", Content: "[]"}}},
		})
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := g.Generate(context.Background(), "", "user only")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(gotReq.Messages) != 1 {
		t.Fatalf("messages count = %d, want 1 (no system)", len(gotReq.Messages))
	}
	if gotReq.Messages[0].Role != "user" {
		t.Fatalf("expected user role, got %q", gotReq.Messages[0].Role)
	}
}

func TestDeepSeekGeneratorReturnsAssistantContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{
				Role:    "assistant",
				Content: `[{"id":"e1","type":"note","source":"director"}]`,
			}}},
		})
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	got, err := g.Generate(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if got != `[{"id":"e1","type":"note","source":"director"}]` {
		t.Fatalf("content = %q", got)
	}
}

func TestDeepSeekGeneratorReturnsErrorOnHTTPFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestDeepSeekGeneratorReturnsErrorOnEmptyChoices(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{Choices: []chatChoice{}})
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error for 0 choices")
	}
}

func TestDeepSeekGeneratorReturnsErrorOnInvalidResponseJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := g.Generate(context.Background(), "", "hi")
	if err == nil {
		t.Fatal("expected error for invalid response JSON")
	}
}

func TestDeepSeekGeneratorUsesDefaults(t *testing.T) {
	t.Parallel()

	var gotReq chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Content: "ok"}}},
		})
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	g.Generate(context.Background(), "", "hi")
	if gotReq.Model != deepseekDefaultModel {
		t.Fatalf("default model = %q, want %q", gotReq.Model, deepseekDefaultModel)
	}
}

func TestDeepSeekGeneratorRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	g := NewDeepSeekGenerator(DeepSeekGeneratorConfig{BaseURL: srv.URL, HTTPClient: srv.Client()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.Generate(ctx, "", "hi")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDeepSeekGeneratorImplementsTextGenerator(t *testing.T) {
	t.Parallel()
	var _ TextGenerator = (*DeepSeekGenerator)(nil)
}
