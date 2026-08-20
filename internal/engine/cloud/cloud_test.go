package cloud_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/cloud"
)

func TestCloudEngine_Interface(t *testing.T) {
	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "gemini-2.5-flash",
	}

	e := cloud.New(cfg)

	// Verify implements engine.Engine interface
	var _ engine.Engine = e

	if e.Name() != "googleai" {
		t.Errorf("expected engine name 'googleai', got %q", e.Name())
	}
}

func TestCloudEngine_MissingAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "gemini-2.5-flash",
	}

	e := cloud.New(cfg)
	_, err := e.Generate(t.Context(), engine.Request{Prompt: "hello"})
	if err == nil {
		t.Error("expected error when API key is missing, got nil")
	}
}

func TestCloudEngine_OllamaNoKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"hello from ollama"}}],"usage":{"total_tokens":5}}`)
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "ollama",
		Model:    "llama3.3",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	resp, err := e.Generate(t.Context(), engine.Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "hello from ollama" {
		t.Errorf("expected 'hello from ollama', got %q", resp.Text)
	}
}

func TestCloudEngine_GeminiGenerate(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Gemini response text"}]}}],"usageMetadata":{"totalTokenCount":12}}`)
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "gemini-2.5-flash",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	resp, err := e.Generate(t.Context(), engine.Request{Prompt: "Explain Go channels"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Gemini response text" {
		t.Errorf("expected 'Gemini response text', got %q", resp.Text)
	}
	if resp.TokensUsed != 12 {
		t.Errorf("expected 12 tokens, got %d", resp.TokensUsed)
	}
}

func TestCloudEngine_GeminiStream(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"candidates":[{"content":{"parts":[{"text":"Hello "}]}}]}`)
		flusher.Flush()

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"candidates":[{"content":{"parts":[{"text":"World!"}]}}],"usageMetadata":{"totalTokenCount":8}}`)
		flusher.Flush()
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "gemini-2.5-flash",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	stream, err := e.GenerateStream(t.Context(), engine.Request{Prompt: "say hello"})
	if err != nil {
		t.Fatalf("unexpected error starting stream: %v", err)
	}

	var accumulated strings.Builder
	var totalTokens int

	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.Final {
			totalTokens = chunk.TokensUsed
		}
		accumulated.WriteString(chunk.Text)
	}

	if accumulated.String() != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", accumulated.String())
	}
	if totalTokens != 8 {
		t.Errorf("expected 8 tokens, got %d", totalTokens)
	}
}

func TestCloudEngine_OpenAIGenerate(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-openai-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"OpenAI answer"}}],"usage":{"total_tokens":25}}`)
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "openai",
		Model:    "gpt-4o",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	resp, err := e.Generate(t.Context(), engine.Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "OpenAI answer" {
		t.Errorf("expected 'OpenAI answer', got %q", resp.Text)
	}
	if resp.TokensUsed != 25 {
		t.Errorf("expected 25 tokens, got %d", resp.TokensUsed)
	}
}

func TestCloudEngine_OpenAIStream(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"Token 1, "}}]}`)
		flusher.Flush()

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"Token 2"}}]}`)
		flusher.Flush()

		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "openai",
		Model:    "gpt-4o",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	stream, err := e.GenerateStream(t.Context(), engine.Request{Prompt: "stream test"})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	var accumulated strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		accumulated.WriteString(chunk.Text)
	}

	if accumulated.String() != "Token 1, Token 2" {
		t.Errorf("expected 'Token 1, Token 2', got %q", accumulated.String())
	}
}

func TestCloudEngine_AnthropicGenerate(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-claude-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-claude-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"content":[{"type":"text","text":"Claude response"}],"usage":{"input_tokens":10,"output_tokens":15}}`)
	}))
	defer server.Close()

	cfg := config.CloudConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet",
		BaseURL:  server.URL,
	}

	e := cloud.New(cfg)
	resp, err := e.Generate(t.Context(), engine.Request{Prompt: "hello claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Claude response" {
		t.Errorf("expected 'Claude response', got %q", resp.Text)
	}
	if resp.TokensUsed != 25 {
		t.Errorf("expected 25 tokens, got %d", resp.TokensUsed)
	}
}
