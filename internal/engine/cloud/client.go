package cloud

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/shell"
)

// Provider interface implemented by all cloud backend adapters.
type Provider interface {
	Generate(ctx context.Context, req engine.Request) (*engine.Response, error)
	GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error)
}

// Engine implements engine.Engine using native HTTP REST clients (zero third-party SDKs).
type Engine struct {
	cfg         config.CloudConfig
	fullCfg     *config.Config
	httpClient  *http.Client
	toolHandler *shell.ToolHandler
}

// New creates a new cloud engine instance.
func New(cfg config.CloudConfig, fullCfg ...*config.Config) *Engine {
	var fc *config.Config
	if len(fullCfg) > 0 {
		fc = fullCfg[0]
	}

	return &Engine{
		cfg:     cfg,
		fullCfg: fc,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		toolHandler: shell.NewToolHandler(fc),
	}
}

// Name returns the engine identifier.
func (e *Engine) Name() string {
	if e.cfg.Provider != "" {
		return e.cfg.Provider
	}
	return "cloud"
}

// resolveAPIKey resolves the API key from config or environment variables.
func (e *Engine) resolveAPIKey() (string, error) {
	apiKey := e.cfg.APIKey

	apiKeyEnv := e.cfg.APIKeyEnv
	if apiKeyEnv == "" {
		switch strings.ToLower(e.cfg.Provider) {
		case "anthropic", "claude":
			apiKeyEnv = "ANTHROPIC_API_KEY"
		case "openai":
			apiKeyEnv = "OPENAI_API_KEY"
		case "groq":
			apiKeyEnv = "GROQ_API_KEY"
		case "deepseek":
			apiKeyEnv = "DEEPSEEK_API_KEY"
		case "ollama":
			apiKeyEnv = ""
		default:
			apiKeyEnv = "GEMINI_API_KEY"
		}
	}

	if apiKey == "" && apiKeyEnv != "" {
		apiKey = os.Getenv(apiKeyEnv)
	}

	// Fallbacks for Gemini
	if apiKey == "" && (strings.ToLower(e.cfg.Provider) == "googleai" || strings.ToLower(e.cfg.Provider) == "gemini" || e.cfg.Provider == "") {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
	}

	// Ollama does not require an API key
	if apiKey == "" && strings.ToLower(e.cfg.Provider) == "ollama" {
		return "ollama", nil
	}

	if apiKey == "" {
		if apiKeyEnv != "" {
			return "", fmt.Errorf("cloud engine: provider %q requires setting %s in the environment", e.cfg.Provider, apiKeyEnv)
		}
		return "", fmt.Errorf("cloud engine: missing API key for provider %q", e.cfg.Provider)
	}

	return apiKey, nil
}

// getProvider resolves and instantiates the proper provider adapter.
func (e *Engine) getProvider() (Provider, error) {
	apiKey, err := e.resolveAPIKey()
	if err != nil {
		return nil, err
	}

	providerName := strings.ToLower(e.cfg.Provider)
	if providerName == "" {
		providerName = "googleai"
	}

	switch providerName {
	case "googleai", "gemini", "google":
		return newGeminiAdapter(e.httpClient, e.cfg, apiKey, e.toolHandler), nil
	case "anthropic", "claude":
		return newAnthropicAdapter(e.httpClient, e.cfg, apiKey, e.toolHandler), nil
	default:
		// Default to universal OpenAI-compatible endpoint (OpenAI, Ollama, Groq, DeepSeek, OpenRouter)
		return newOpenAIAdapter(e.httpClient, e.cfg, apiKey, providerName, e.toolHandler), nil
	}
}

// Generate executes a synchronous text completion.
func (e *Engine) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	p, err := e.getProvider()
	if err != nil {
		return nil, err
	}
	return p.Generate(ctx, req)
}

// GenerateStream yields incremental tokens over a channel as they stream via SSE.
func (e *Engine) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	p, err := e.getProvider()
	if err != nil {
		return nil, err
	}
	return p.GenerateStream(ctx, req)
}

// Close cleans up engine resources.
func (e *Engine) Close() error {
	return nil
}
