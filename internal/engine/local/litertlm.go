package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/vladimirvivien/litertlm-go/pkg/litertlm"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

// Engine implements engine.Engine using Google LiteRT-LM.
type Engine struct {
	cfg    config.LocalConfig
	client *litertlm.Client
	mu     sync.Mutex
}

// New creates a new LiteRT-LM local engine instance.
func New(cfg config.LocalConfig) *Engine {
	return &Engine{
		cfg: cfg,
	}
}

// Name returns the engine identifier.
func (e *Engine) Name() string {
	return "litertlm"
}

// Client returns the underlying *litertlm.Client, initializing it if necessary.
func (e *Engine) Client(ctx context.Context) (*litertlm.Client, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client != nil {
		return e.client, nil
	}

	libDir := e.cfg.LibDir
	modelPath := e.cfg.Model

	// Auto-provision native shared libraries if needed
	if libDir == "" && e.cfg.AutoDownload {
		libVersion := e.cfg.LibVersion
		if libVersion == "" {
			libVersion = config.DefaultLocalLibVersion
		}
		dir, err := litertlm.FetchLib(runtime.GOOS, runtime.GOARCH, libVersion)
		if err != nil {
			return nil, fmt.Errorf("local: fetch lib: %w", err)
		}
		libDir = dir
	}

	// Auto-provision model if needed
	if !filepath.IsAbs(modelPath) && !fileExists(modelPath) && e.cfg.AutoDownload {
		cachedPath, err := litertlm.FetchModel(ctx, modelPath)
		if err != nil {
			return nil, fmt.Errorf("local: fetch model: %w", err)
		}
		modelPath = cachedPath
	}

	opts := []litertlm.Option{
		litertlm.WithModel(modelPath),
	}
	if libDir != "" {
		opts = append(opts, litertlm.WithLib(libDir))
	}
	if e.cfg.Backend != "" {
		opts = append(opts, litertlm.WithBackend(e.cfg.Backend))
	}
	if e.cfg.CacheDir != "" {
		opts = append(opts, litertlm.WithCacheDir(e.cfg.CacheDir))
	}

	client, err := litertlm.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("local: initialize litertlm: %w", err)
	}

	e.client = client
	return client, nil
}

// Generate executes a synchronous text completion.
func (e *Engine) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	client, err := e.Client(ctx)
	if err != nil {
		return nil, err
	}

	fullPrompt := formatPrompt(req)

	// If a system prompt is provided, run through Chat to frame correctly
	if req.SystemPrompt != "" {
		chat, err := client.NewChat(ctx, litertlm.WithSystemPrompt(req.SystemPrompt))
		if err != nil {
			return nil, fmt.Errorf("local: new chat: %w", err)
		}
		defer func() { _ = chat.Close() }()

		reply, err := chat.Send(ctx, fullPrompt)
		if err != nil {
			return nil, fmt.Errorf("local: chat send: %w", err)
		}

		return &engine.Response{
			Text:      reply.Text(),
			Provider:  "litertlm",
			Model:     e.cfg.Model,
			UsedLocal: true,
		}, nil
	}

	var runtimeOpts []litertlm.RuntimeOption
	if req.MaxTokens > 0 {
		runtimeOpts = append(runtimeOpts, litertlm.WithMaxOutputTokens(req.MaxTokens))
	}

	text, err := client.Generate(ctx, fullPrompt, runtimeOpts...)
	if err != nil {
		return nil, fmt.Errorf("local: generate: %w", err)
	}

	return &engine.Response{
		Text:      text,
		Provider:  "litertlm",
		Model:     e.cfg.Model,
		UsedLocal: true,
	}, nil
}

// GenerateStream yields tokens over a channel as they are emitted by LiteRT-LM.
func (e *Engine) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	client, err := e.Client(ctx)
	if err != nil {
		return nil, err
	}

	fullPrompt := formatPrompt(req)
	out := make(chan engine.StreamChunk, 20)

	var runtimeOpts []litertlm.RuntimeOption
	if req.MaxTokens > 0 {
		runtimeOpts = append(runtimeOpts, litertlm.WithMaxOutputTokens(req.MaxTokens))
	}

	go func() {
		defer close(out)

		if req.SystemPrompt != "" {
			chat, err := client.NewChat(ctx, litertlm.WithSystemPrompt(req.SystemPrompt))
			if err != nil {
				out <- engine.StreamChunk{Error: fmt.Errorf("local: new chat: %w", err)}
				return
			}
			defer func() { _ = chat.Close() }()

			for chunk, err := range chat.SendStream(ctx, fullPrompt, runtimeOpts...) {
				if err != nil {
					out <- engine.StreamChunk{Error: err}
					return
				}
				out <- engine.StreamChunk{
					Text:  chunk.Text,
					Final: chunk.Final,
				}
			}
			return
		}

		for chunk, err := range client.GenerateStream(ctx, fullPrompt, runtimeOpts...) {
			if err != nil {
				out <- engine.StreamChunk{Error: err}
				return
			}
			out <- engine.StreamChunk{
				Text:  chunk.Text,
				Final: chunk.Final,
			}
		}
	}()

	return out, nil
}

// Close terminates the engine and frees native model allocations.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client != nil {
		err := e.client.Close()
		e.client = nil
		return err
	}
	return nil
}

func formatPrompt(req engine.Request) string {
	var sb strings.Builder

	if len(req.ContextFiles) > 0 {
		sb.WriteString("Context files:\n")
		for _, f := range req.ContextFiles {
			fmt.Fprintf(&sb, "--- File: %s ---\n%s\n--- End File ---\n\n", f.Path, f.Content)
		}
	}

	sb.WriteString(req.Prompt)
	return sb.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
