package daemon

import "github.com/vladimirvivien/robo/internal/engine"

// GenerateRequest is the JSON payload sent to /v1/generate endpoints.
type GenerateRequest struct {
	Prompt       string                `json:"prompt"`
	SystemPrompt string                `json:"system_prompt,omitempty"`
	ContextFiles []engine.FileContext  `json:"context_files,omitempty"`
	Images       []engine.ImageContext `json:"images,omitempty"`
	MaxTokens    int                   `json:"max_tokens,omitempty"`
	Temperature  float64               `json:"temperature,omitempty"`
}

// GenerateResponse is the JSON payload returned by /v1/generate.
type GenerateResponse struct {
	Text       string `json:"text"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	UsedLocal  bool   `json:"used_local"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Error      string `json:"error,omitempty"`
}

// StreamChunkPayload is the JSON payload serialized into SSE data: lines.
type StreamChunkPayload struct {
	Text       string `json:"text,omitempty"`
	Final      bool   `json:"final,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Error      string `json:"error,omitempty"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
	Model  string `json:"model"`
	PID    int    `json:"pid"`
}
