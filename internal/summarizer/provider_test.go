package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestCodexHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("Authorization = %q, want Bearer oauth-token", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "ok",
					},
				},
			},
		})
	}))
	defer server.Close()

	s := &CodexSummarizer{
		Model:              "codex",
		OAuthToken:         "oauth-token",
		ChatCompletionsURL: server.URL,
		CodexResponsesURL:  server.URL + "/unused",
		HTTPClient:         server.Client(),
	}

	text, err := s.Summarize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if text != "ok" {
		t.Fatalf("text = %q, want ok", text)
	}
}

func TestModelFallbackOrder(t *testing.T) {
	var attempted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error: %v", err)
		}
		attempted = append(attempted, req.Model)
		if req.Model != "gpt-5.2-codex" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"model unavailable"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{"content": "fallback ok"},
				},
			},
		})
	}))
	defer server.Close()

	s := &CodexSummarizer{
		Model:              "codex",
		OAuthToken:         "oauth-token",
		ChatCompletionsURL: server.URL,
		CodexResponsesURL:  server.URL + "/unused",
		HTTPClient:         server.Client(),
	}

	text, err := s.Summarize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if text != "fallback ok" {
		t.Fatalf("text = %q, want fallback ok", text)
	}

	want := []string{"codex", "gpt-5.3-codex", "gpt-5.2-codex"}
	if !slices.Equal(attempted, want) {
		t.Fatalf("attempted = %#v, want %#v", attempted, want)
	}
}

func TestTransportFallbackOnMissingScope(t *testing.T) {
	chatCalls := 0
	fallbackCalls := 0

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatCalls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Missing scopes: model.request"}}`))
	}))
	defer chatServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("Authorization = %q, want Bearer oauth-token", got)
		}

		var req codexResponsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error: %v", err)
		}
		if strings.TrimSpace(req.Instructions) == "" {
			t.Fatal("instructions should not be empty")
		}
		if len(req.Input) == 0 {
			t.Fatal("input should be a non-empty list")
		}
		if !req.Stream {
			t.Fatal("stream = false, want true")
		}
		if req.Store {
			t.Fatal("store = true, want false")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello \"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"codex\"}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer fallbackServer.Close()

	s := &CodexSummarizer{
		Model:              "codex",
		OAuthToken:         "oauth-token",
		ChatCompletionsURL: chatServer.URL,
		CodexResponsesURL:  fallbackServer.URL,
		HTTPClient:         chatServer.Client(),
	}

	text, err := s.Summarize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if text != "hello codex" {
		t.Fatalf("text = %q, want hello codex", text)
	}
	if chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", chatCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
}

func TestFallbackMapsDeveloperAndSystemToInstructions(t *testing.T) {
	var got codexResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode() error: %v", err)
		}
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	defer server.Close()

	s := &CodexSummarizer{
		Model:             "codex",
		OAuthToken:        "oauth-token",
		CodexResponsesURL: server.URL,
		HTTPClient:        server.Client(),
	}

	_, err := s.callCodexResponsesFallback(context.Background(), chatCompletionRequest{
		Model: "codex",
		Messages: []chatMessage{
			{Role: "system", Content: "system rule"},
			{Role: "developer", Content: "developer rule"},
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("callCodexResponsesFallback() error: %v", err)
	}

	if !strings.Contains(got.Instructions, "system rule") || !strings.Contains(got.Instructions, "developer rule") {
		t.Fatalf("instructions = %q, want both system and developer content", got.Instructions)
	}
	if len(got.Input) != 1 || len(got.Input[0].Content) != 1 || got.Input[0].Content[0].Text != "hello" {
		t.Fatalf("input = %#v, want one input message with hello text", got.Input)
	}
}

func TestDebugLoggingIncludesPayloads(t *testing.T) {
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Missing scopes: model.request"}}`))
	}))
	defer chatServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	defer fallbackServer.Close()

	var logs bytes.Buffer
	s := &CodexSummarizer{
		Model:              "codex",
		OAuthToken:         "oauth-token",
		ChatCompletionsURL: chatServer.URL,
		CodexResponsesURL:  fallbackServer.URL,
		HTTPClient:         chatServer.Client(),
		Debug:              true,
		DebugWriter:        &logs,
	}

	_, err := s.Summarize(context.Background(), "hello debug")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}

	text := logs.String()
	if !strings.Contains(text, "chat.completions request") {
		t.Fatalf("missing chat.completions debug log: %s", text)
	}
	if !strings.Contains(text, "codex.responses request") {
		t.Fatalf("missing codex.responses debug log: %s", text)
	}
}

func TestTransportFallbackParsesSSEWithoutWaitingForConnectionClose(t *testing.T) {
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Missing scopes: model.request"}}`))
	}))
	defer chatServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"delta\":\"streamed \"}\n\n"))
		_, _ = w.Write([]byte("data: {\"delta\":\"result\"}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer fallbackServer.Close()

	s := &CodexSummarizer{
		Model:              "gpt-5.3-codex",
		OAuthToken:         "oauth-token",
		ChatCompletionsURL: chatServer.URL,
		CodexResponsesURL:  fallbackServer.URL,
		HTTPClient:         chatServer.Client(),
	}

	text, err := s.Summarize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if text != "streamed result" {
		t.Fatalf("text = %q, want streamed result", text)
	}
}
