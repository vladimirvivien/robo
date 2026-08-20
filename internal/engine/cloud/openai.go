package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/shell"
)

type openAIAdapter struct {
	client      *http.Client
	cfg         config.CloudConfig
	apiKey      string
	provider    string
	toolHandler *shell.ToolHandler
}

func newOpenAIAdapter(client *http.Client, cfg config.CloudConfig, apiKey string, provider string, toolHandler *shell.ToolHandler) *openAIAdapter {
	return &openAIAdapter{
		client:      client,
		cfg:         cfg,
		apiKey:      apiKey,
		provider:    provider,
		toolHandler: toolHandler,
	}
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIFunctionDecl `json:"function"`
}

type openAIFunctionDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIUsage struct {
	TotalTokens int `json:"total_tokens"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code,omitempty"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage"`
}

func (a *openAIAdapter) getBaseURL() string {
	if a.cfg.BaseURL != "" {
		return strings.TrimSuffix(a.cfg.BaseURL, "/")
	}

	switch strings.ToLower(a.provider) {
	case "ollama":
		return "http://localhost:11434/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func (a *openAIAdapter) normalizeModel(model string) string {
	if model == "" {
		switch strings.ToLower(a.provider) {
		case "ollama":
			return "llama3.3"
		case "groq":
			return "llama-3.3-70b-versatile"
		case "deepseek":
			return "deepseek-chat"
		default:
			return "gpt-4o"
		}
	}
	model = strings.TrimPrefix(model, "openai/")
	model = strings.TrimPrefix(model, "ollama/")
	return model
}

func (a *openAIAdapter) buildRequest(req engine.Request, stream bool) openAIRequest {
	model := a.normalizeModel(a.cfg.Model)

	var messages []openAIMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	var userPrompt strings.Builder
	if len(req.ContextFiles) > 0 {
		userPrompt.WriteString("Context files:\n")
		for _, f := range req.ContextFiles {
			fmt.Fprintf(&userPrompt, "--- File: %s ---\n%s\n--- End File ---\n\n", f.Path, f.Content)
		}
	}
	userPrompt.WriteString(req.Prompt)

	messages = append(messages, openAIMessage{
		Role:    "user",
		Content: userPrompt.String(),
	})

	oReq := openAIRequest{
		Model:    model,
		Messages: messages,
		Stream:   stream,
	}

	if req.Temperature > 0 {
		temp := req.Temperature
		oReq.Temperature = &temp
	}
	if req.MaxTokens > 0 {
		maxTok := req.MaxTokens
		oReq.MaxTokens = &maxTok
	}

	if a.toolHandler != nil {
		oReq.Tools = []openAITool{
			{
				Type: "function",
				Function: openAIFunctionDecl{
					Name:        "execute_shell",
					Description: "Propose and execute a shell command on the host operating system.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"command": map[string]any{
								"type":        "string",
								"description": "The exact shell command or script to execute.",
							},
							"description": map[string]any{
								"type":        "string",
								"description": "Brief explanation of what the command does.",
							},
						},
						"required": []string{"command"},
					},
				},
			},
		}
	}

	return oReq
}

func (a *openAIAdapter) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	url := fmt.Sprintf("%s/chat/completions", a.getBaseURL())

	oReq := a.buildRequest(req, false)
	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var oResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	if oResp.Error != nil {
		return nil, fmt.Errorf("openai API error (%v): %s", oResp.Error.Code, oResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: unexpected HTTP status: %d", resp.StatusCode)
	}

	var fullText strings.Builder
	tokens := 0
	if oResp.Usage != nil {
		tokens = oResp.Usage.TotalTokens
	}

	for _, choice := range oResp.Choices {
		if choice.Message.Content != "" {
			fullText.WriteString(choice.Message.Content)
		}
		for _, tc := range choice.Message.ToolCalls {
			if tc.Function.Name == "execute_shell" && a.toolHandler != nil {
				var input shell.ShellInput
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				out, err := a.toolHandler.Handle(ctx, input)
				if err == nil && out.Output != "" {
					fullText.WriteString("\n" + out.Output)
				}
			}
		}
	}

	return &engine.Response{
		Text:       fullText.String(),
		Provider:   a.provider,
		Model:      oReq.Model,
		UsedLocal:  false,
		TokensUsed: tokens,
	}, nil
}

func (a *openAIAdapter) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	url := fmt.Sprintf("%s/chat/completions", a.getBaseURL())

	oReq := a.buildRequest(req, true)
	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http post: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		var oResp openAIResponse
		_ = json.NewDecoder(resp.Body).Decode(&oResp)
		if oResp.Error != nil {
			return nil, fmt.Errorf("openai API error (%v): %s", oResp.Error.Code, oResp.Error.Message)
		}
		return nil, fmt.Errorf("openai: unexpected HTTP status: %d", resp.StatusCode)
	}

	out := make(chan engine.StreamChunk, 20)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(out)

		totalTokens := 0
		var toolCallArgs strings.Builder
		var toolCallName string

		err := ReadSSE(resp.Body, func(ev SSEEvent) error {
			if len(ev.Data) == 0 {
				return nil
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal(ev.Data, &chunk); err != nil {
				return nil
			}

			if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
				totalTokens = chunk.Usage.TotalTokens
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					out <- engine.StreamChunk{Text: choice.Delta.Content}
				}
				for _, tc := range choice.Delta.ToolCalls {
					if tc.Function.Name != "" {
						toolCallName = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						toolCallArgs.WriteString(tc.Function.Arguments)
					}
				}
			}
			return nil
		})

		if err != nil {
			out <- engine.StreamChunk{Error: fmt.Errorf("openai: stream error: %w", err)}
			return
		}

		// Execute tool call if accumulated during stream
		if toolCallName == "execute_shell" && a.toolHandler != nil {
			var input shell.ShellInput
			_ = json.Unmarshal([]byte(toolCallArgs.String()), &input)
			toolRes, _ := a.toolHandler.Handle(ctx, input)
			if toolRes.Output != "" {
				out <- engine.StreamChunk{Text: "\n" + toolRes.Output}
			}
		}

		out <- engine.StreamChunk{
			Final:      true,
			TokensUsed: totalTokens,
		}
	}()

	return out, nil
}
