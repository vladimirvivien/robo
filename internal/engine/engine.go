package engine

import (
	"context"
)

// Request defines the unified payload for generating LLM completions.
type Request struct {
	Prompt       string         `json:"prompt"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	ContextFiles []FileContext  `json:"context_files,omitempty"`
	Images       []ImageContext `json:"images,omitempty"`
	MaxTokens    int            `json:"max_tokens,omitempty"`
	Temperature  float64        `json:"temperature,omitempty"`
	ForceBackend string         `json:"force_backend,omitempty"` // "local", "cloud", or "" (auto)
}

// FileContext represents an attached file with path and text content.
type FileContext struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ImageContext represents an attached image with mime type and binary data.
type ImageContext struct {
	Path     string `json:"path,omitempty"`
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

// Response represents a completed model generation.
type Response struct {
	Text       string `json:"text"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	UsedLocal  bool   `json:"used_local"`
	TokensUsed int    `json:"tokens_used,omitempty"`
}

// StreamChunk represents an incremental token emitted during streaming.
type StreamChunk struct {
	Text       string `json:"text,omitempty"`
	Final      bool   `json:"final,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Error      error  `json:"error,omitempty"`
}

// Engine is the core interface implemented by local, cloud, daemon, and router engines.
type Engine interface {
	// Name returns the identifier of this engine (e.g. "litertlm", "genkit", "router").
	Name() string

	// Generate produces a complete text response for the given request.
	Generate(ctx context.Context, req Request) (*Response, error)

	// GenerateStream yields incremental tokens over a channel until completion.
	GenerateStream(ctx context.Context, req Request) (<-chan StreamChunk, error)

	// Close releases any allocated resources.
	Close() error
}
