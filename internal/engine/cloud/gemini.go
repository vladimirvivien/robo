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

type geminiAdapter struct {
	client      *http.Client
	cfg         config.LLMConfig
	apiKey      string
	toolHandler *shell.ToolHandler
}

func newGeminiAdapter(client *http.Client, cfg config.LLMConfig, apiKey string, toolHandler *shell.ToolHandler) *geminiAdapter {
	return &geminiAdapter{
		client:      client,
		cfg:         cfg,
		apiKey:      apiKey,
		toolHandler: toolHandler,
	}
}

// Request and response types for Gemini REST API
type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []geminiToolDeclaration `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse `json:"functionResponse,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

type geminiToolDeclaration struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFuncResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
	Error         *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsage struct {
	TotalTokenCount int `json:"totalTokenCount"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (a *geminiAdapter) normalizeModel(model string) string {
	if model == "" {
		model = config.DefaultCloudModel
	}
	model = strings.TrimPrefix(model, "googleai/")
	model = strings.TrimPrefix(model, "models/")
	return model
}

func (a *geminiAdapter) buildRequest(req engine.Request) geminiRequest {
	var gReq geminiRequest

	if req.SystemPrompt != "" {
		gReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	var userPrompt strings.Builder
	if len(req.ContextFiles) > 0 {
		userPrompt.WriteString("Context files:\n")
		for _, f := range req.ContextFiles {
			fmt.Fprintf(&userPrompt, "--- File: %s ---\n%s\n--- End File ---\n\n", f.Path, f.Content)
		}
	}
	userPrompt.WriteString(req.Prompt)

	gReq.Contents = []geminiContent{
		{
			Role:  "user",
			Parts: []geminiPart{{Text: userPrompt.String()}},
		},
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		gReq.GenerationConfig = &geminiGenerationConfig{}
		if req.Temperature > 0 {
			temp := req.Temperature
			gReq.GenerationConfig.Temperature = &temp
		}
		if req.MaxTokens > 0 {
			maxTok := req.MaxTokens
			gReq.GenerationConfig.MaxOutputTokens = &maxTok
		}
	}

	if a.toolHandler != nil {
		gReq.Tools = []geminiToolDeclaration{
			{
				FunctionDeclarations: []geminiFunctionDecl{
					{
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
			},
		}
	}

	return gReq
}

func (a *geminiAdapter) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	model := a.normalizeModel(a.cfg.Model)
	baseURL := a.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", baseURL, model, a.apiKey)

	gReq := a.buildRequest(req)
	body, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}

	if gResp.Error != nil {
		return nil, fmt.Errorf("gemini API error (code %d): %s", gResp.Error.Code, gResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini: unexpected HTTP status: %d", resp.StatusCode)
	}

	var fullText strings.Builder
	tokens := 0
	if gResp.UsageMetadata != nil {
		tokens = gResp.UsageMetadata.TotalTokenCount
	}

	for _, cand := range gResp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				fullText.WriteString(part.Text)
			}
			if part.FunctionCall != nil && a.toolHandler != nil && part.FunctionCall.Name == "execute_shell" {
				cmdStr, _ := part.FunctionCall.Args["command"].(string)
				desc, _ := part.FunctionCall.Args["description"].(string)
				out, err := a.toolHandler.Handle(ctx, shell.ShellInput{
					Command:     cmdStr,
					Description: desc,
				})
				if err == nil && out.Output != "" {
					fullText.WriteString("\n" + out.Output)
				}
			}
		}
	}

	return &engine.Response{
		Text:       fullText.String(),
		Provider:   "googleai",
		Model:      model,
		UsedLocal:  false,
		TokensUsed: tokens,
	}, nil
}

func (a *geminiAdapter) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	model := a.normalizeModel(a.cfg.Model)
	baseURL := a.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse", baseURL, model, a.apiKey)

	gReq := a.buildRequest(req)
	body, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: http post: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		var gResp geminiResponse
		_ = json.NewDecoder(resp.Body).Decode(&gResp)
		if gResp.Error != nil {
			return nil, fmt.Errorf("gemini API error (code %d): %s", gResp.Error.Code, gResp.Error.Message)
		}
		return nil, fmt.Errorf("gemini: unexpected HTTP status: %d", resp.StatusCode)
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

			var chunk geminiResponse
			if err := json.Unmarshal(ev.Data, &chunk); err != nil {
				return nil
			}

			if chunk.UsageMetadata != nil && chunk.UsageMetadata.TotalTokenCount > 0 {
				totalTokens = chunk.UsageMetadata.TotalTokenCount
			}

			for _, cand := range chunk.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						out <- engine.StreamChunk{Text: part.Text}
					}
					if part.FunctionCall != nil && a.toolHandler != nil && part.FunctionCall.Name == "execute_shell" {
						cmdStr, _ := part.FunctionCall.Args["command"].(string)
						desc, _ := part.FunctionCall.Args["description"].(string)
						toolRes, _ := a.toolHandler.Handle(ctx, shell.ShellInput{
							Command:     cmdStr,
							Description: desc,
						})
						if toolRes.Output != "" {
							out <- engine.StreamChunk{Text: "\n" + toolRes.Output}
						}
					}
				}
			}
			return nil
		})

		if err != nil {
			out <- engine.StreamChunk{Error: fmt.Errorf("gemini: stream error: %w", err)}
			return
		}

		out <- engine.StreamChunk{
			Final:      true,
			TokensUsed: totalTokens,
		}
	}()

	return out, nil
}
