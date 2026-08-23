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

type anthropicAdapter struct {
	client      *http.Client
	cfg         config.LLMConfig
	apiKey      string
	toolHandler *shell.ToolHandler
}

func newAnthropicAdapter(client *http.Client, cfg config.LLMConfig, apiKey string, toolHandler *shell.ToolHandler) *anthropicAdapter {
	return &anthropicAdapter{
		client:      client,
		cfg:         cfg,
		apiKey:      apiKey,
		toolHandler: toolHandler,
	}
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"` // "user", "assistant"
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"` // "text", "tool_use"
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   *anthropicUsage         `json:"usage,omitempty"`
	Error   *anthropicError         `json:"error,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Usage *anthropicUsage `json:"usage"`
}

func (a *anthropicAdapter) normalizeModel(model string) string {
	if model == "" {
		return "claude-3-5-sonnet-20241022"
	}
	model = strings.TrimPrefix(model, "anthropic/")
	model = strings.TrimPrefix(model, "claude/")
	return model
}

func (a *anthropicAdapter) buildRequest(req engine.Request, stream bool) anthropicRequest {
	model := a.normalizeModel(a.cfg.Model)

	var userPrompt strings.Builder
	if len(req.ContextFiles) > 0 {
		userPrompt.WriteString("Context files:\n")
		for _, f := range req.ContextFiles {
			fmt.Fprintf(&userPrompt, "--- File: %s ---\n%s\n--- End File ---\n\n", f.Path, f.Content)
		}
	}
	userPrompt.WriteString(req.Prompt)

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	aReq := anthropicRequest{
		Model: model,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: userPrompt.String(),
			},
		},
		System:    req.SystemPrompt,
		MaxTokens: maxTokens,
		Stream:    stream,
	}

	if req.Temperature > 0 {
		temp := req.Temperature
		aReq.Temperature = &temp
	}

	if a.toolHandler != nil {
		aReq.Tools = []anthropicTool{
			{
				Name:        "execute_shell",
				Description: "Propose and execute a shell command on the host operating system.",
				InputSchema: map[string]any{
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
		}
	}

	return aReq
}

func (a *anthropicAdapter) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	baseURL := a.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	url := fmt.Sprintf("%s/messages", strings.TrimSuffix(baseURL, "/"))

	aReq := a.buildRequest(req, false)
	body, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var aResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&aResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	if aResp.Error != nil {
		return nil, fmt.Errorf("anthropic API error (%s): %s", aResp.Error.Type, aResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: unexpected HTTP status: %d", resp.StatusCode)
	}

	var fullText strings.Builder
	tokens := 0
	if aResp.Usage != nil {
		tokens = aResp.Usage.InputTokens + aResp.Usage.OutputTokens
	}

	for _, block := range aResp.Content {
		if block.Type == "text" {
			fullText.WriteString(block.Text)
		} else if block.Type == "tool_use" && block.Name == "execute_shell" && a.toolHandler != nil {
			cmdStr, _ := block.Input["command"].(string)
			desc, _ := block.Input["description"].(string)
			out, err := a.toolHandler.Handle(ctx, shell.ShellInput{
				Command:     cmdStr,
				Description: desc,
			})
			if err == nil && out.Output != "" {
				fullText.WriteString("\n" + out.Output)
			}
		}
	}

	return &engine.Response{
		Text:       fullText.String(),
		Provider:   "anthropic",
		Model:      aReq.Model,
		UsedLocal:  false,
		TokensUsed: tokens,
	}, nil
}

func (a *anthropicAdapter) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	baseURL := a.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	url := fmt.Sprintf("%s/messages", strings.TrimSuffix(baseURL, "/"))

	aReq := a.buildRequest(req, true)
	body, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http post: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		var aResp anthropicResponse
		_ = json.NewDecoder(resp.Body).Decode(&aResp)
		if aResp.Error != nil {
			return nil, fmt.Errorf("anthropic API error (%s): %s", aResp.Error.Type, aResp.Error.Message)
		}
		return nil, fmt.Errorf("anthropic: unexpected HTTP status: %d", resp.StatusCode)
	}

	out := make(chan engine.StreamChunk, 20)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(out)

		totalTokens := 0

		err := ReadSSE(resp.Body, func(ev SSEEvent) error {
			if len(ev.Data) == 0 {
				return nil
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal(ev.Data, &event); err != nil {
				return nil
			}

			if event.Delta.Text != "" {
				out <- engine.StreamChunk{Text: event.Delta.Text}
			}

			if event.Usage != nil {
				totalTokens += event.Usage.OutputTokens
			}

			return nil
		})

		if err != nil {
			out <- engine.StreamChunk{Error: fmt.Errorf("anthropic: stream error: %w", err)}
			return
		}

		out <- engine.StreamChunk{
			Final:      true,
			TokensUsed: totalTokens,
		}
	}()

	return out, nil
}
