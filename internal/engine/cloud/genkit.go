package cloud

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

// Engine implements engine.Engine using the Genkit Go SDK.
type Engine struct {
	cfg config.CloudConfig
	g   *genkit.Genkit
	mu  sync.Mutex
}

// New creates a new Genkit cloud engine instance.
func New(cfg config.CloudConfig) *Engine {
	return &Engine{
		cfg: cfg,
	}
}

// Name returns the engine identifier.
func (e *Engine) Name() string {
	return "genkit"
}

// Genkit returns the initialized Genkit instance, lazily initializing provider plugins.
func (e *Engine) Genkit(ctx context.Context) (resG *genkit.Genkit, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.g != nil {
		return e.g, nil
	}

	apiKey := e.cfg.APIKey
	if apiKey == "" && e.cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(e.cfg.APIKeyEnv)
	}

	expectedKeyEnv := e.cfg.APIKeyEnv
	if expectedKeyEnv == "" {
		switch strings.ToLower(e.cfg.Provider) {
		case "anthropic", "claude":
			expectedKeyEnv = "ANTHROPIC_API_KEY"
		case "openai":
			expectedKeyEnv = "OPENAI_API_KEY"
		default:
			expectedKeyEnv = "GEMINI_API_KEY"
		}
	}

	if apiKey == "" && expectedKeyEnv != "" {
		apiKey = os.Getenv(expectedKeyEnv)
	}
	if apiKey == "" && (strings.ToLower(e.cfg.Provider) == "googleai" || strings.ToLower(e.cfg.Provider) == "gemini" || e.cfg.Provider == "") {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("cloud engine: provider %q requires setting %s in the environment", e.cfg.Provider, expectedKeyEnv)
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cloud engine initialization: %v", r)
		}
	}()

	var plugins []api.Plugin

	switch strings.ToLower(e.cfg.Provider) {
	case "googleai", "gemini", "":
		_ = os.Setenv("GEMINI_API_KEY", apiKey)
		plugins = append(plugins, &googlegenai.GoogleAI{APIKey: apiKey})
	default:
		// Default to GoogleAI plugin
		_ = os.Setenv("GEMINI_API_KEY", apiKey)
		plugins = append(plugins, &googlegenai.GoogleAI{APIKey: apiKey})
	}

	modelName := e.cfg.Model
	if modelName == "" {
		modelName = config.DefaultCloudModel
	}

	g := genkit.Init(ctx,
		genkit.WithPlugins(plugins...),
		genkit.WithDefaultModel(modelName),
	)

	e.g = g
	return g, nil
}

// Generate executes a synchronous text completion using Genkit.
func (e *Engine) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	g, err := e.Genkit(ctx)
	if err != nil {
		return nil, err
	}

	opts := e.buildGenkitOptions(req)

	resp, err := genkit.Generate(ctx, g, opts...)
	if err != nil {
		return nil, fmt.Errorf("cloud: generate: %w", err)
	}

	modelName := e.cfg.Model
	if modelName == "" {
		modelName = config.DefaultCloudModel
	}

	return &engine.Response{
		Text:      resp.Text(),
		Provider:  e.cfg.Provider,
		Model:     modelName,
		UsedLocal: false,
	}, nil
}

// GenerateStream yields tokens over a channel as they stream from Genkit.
func (e *Engine) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	g, err := e.Genkit(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan engine.StreamChunk, 20)
	opts := e.buildGenkitOptions(req)

	go func() {
		defer close(out)

		for result, err := range genkit.GenerateStream(ctx, g, opts...) {
			if err != nil {
				out <- engine.StreamChunk{Error: fmt.Errorf("cloud: stream: %w", err)}
				return
			}
			if result.Done {
				tokens := 0
				if result.Response != nil {
					tokens = len(result.Response.Text()) / 4
				}
				out <- engine.StreamChunk{
					Final:      true,
					TokensUsed: tokens,
				}
				return
			}
			if result.Chunk != nil {
				out <- engine.StreamChunk{
					Text: result.Chunk.Text(),
				}
			}
		}
	}()

	return out, nil
}

// Close cleans up Genkit resources.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.g = nil
	return nil
}

func (e *Engine) buildGenkitOptions(req engine.Request) []ai.GenerateOption {
	var opts []ai.GenerateOption

	modelName := e.cfg.Model
	if modelName == "" {
		modelName = config.DefaultCloudModel
	}
	opts = append(opts, ai.WithModelName(modelName))

	var messages []*ai.Message
	if req.SystemPrompt != "" {
		messages = append(messages, ai.NewSystemTextMessage(req.SystemPrompt))
	}

	// Build context parts
	var promptText strings.Builder
	if len(req.ContextFiles) > 0 {
		promptText.WriteString("Context files:\n")
		for _, f := range req.ContextFiles {
			fmt.Fprintf(&promptText, "--- File: %s ---\n%s\n--- End File ---\n\n", f.Path, f.Content)
		}
	}
	promptText.WriteString(req.Prompt)

	messages = append(messages, ai.NewUserTextMessage(promptText.String()))
	opts = append(opts, ai.WithMessages(messages...))

	if req.Temperature > 0 {
		cfg := map[string]any{
			"temperature": req.Temperature,
		}
		opts = append(opts, ai.WithConfig(cfg))
	}

	return opts
}
