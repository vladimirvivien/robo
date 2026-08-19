package local

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/vladimirvivien/litertlm-go/pkg/litertlm"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/shell"
)

// Engine implements engine.Engine using Google LiteRT-LM.
type Engine struct {
	cfg       config.LocalConfig
	fullCfg   *config.Config
	client    *litertlm.Client
	shellTool *litertlm.ManagedTool[shell.ShellInput, shell.ShellOutput]
	mu        sync.Mutex
}

// New creates a new LiteRT-LM local engine instance.
func New(cfg config.LocalConfig, fullCfg ...*config.Config) *Engine {
	var fc *config.Config
	if len(fullCfg) > 0 {
		fc = fullCfg[0]
	}
	return &Engine{
		cfg:     cfg,
		fullCfg: fc,
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

	libDir, modelPath, err := engine.EnsureLocalSetup(ctx, e.cfg)
	if err != nil {
		return nil, err
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
		if err := os.MkdirAll(e.cfg.CacheDir, 0750); err != nil {
			return nil, fmt.Errorf("local: create cache dir: %w", err)
		}
		opts = append(opts, litertlm.WithCacheDir(e.cfg.CacheDir))
	}

	client, err := litertlm.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("local: initialize litertlm: %w", err)
	}

	toolHandler := shell.NewToolHandler(e.fullCfg)
	shellTool, err := litertlm.RegisterTool(client, "execute_shell",
		"Propose and execute a shell command on the host operating system.",
		func(ctx context.Context, in shell.ShellInput) (shell.ShellOutput, error) {
			return toolHandler.Handle(ctx, in)
		},
	)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("local: register shell tool: %w", err)
	}

	e.client = client
	e.shellTool = shellTool
	return client, nil
}

// Generate executes a synchronous text completion.
func (e *Engine) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	client, err := e.Client(ctx)
	if err != nil {
		return nil, err
	}

	fullPrompt := formatPrompt(req)

	var chatOpts []litertlm.ChatOption
	if req.SystemPrompt != "" {
		chatOpts = append(chatOpts, litertlm.WithSystemPrompt(req.SystemPrompt))
	}
	if e.shellTool != nil {
		chatOpts = append(chatOpts, litertlm.WithTool(e.shellTool))
	}

	chat, err := client.NewChat(ctx, chatOpts...)
	if err != nil {
		return nil, fmt.Errorf("local: new chat: %w", err)
	}
	defer func() { _ = chat.Close() }()

	var runtimeOpts []litertlm.RuntimeOption
	if req.MaxTokens > 0 {
		runtimeOpts = append(runtimeOpts, litertlm.WithMaxOutputTokens(req.MaxTokens))
	}

	reply, err := chat.Send(ctx, fullPrompt, runtimeOpts...)
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

		var chatOpts []litertlm.ChatOption
		if req.SystemPrompt != "" {
			chatOpts = append(chatOpts, litertlm.WithSystemPrompt(req.SystemPrompt))
		}
		if e.shellTool != nil {
			chatOpts = append(chatOpts, litertlm.WithTool(e.shellTool))
		}

		chat, err := client.NewChat(ctx, chatOpts...)
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
		e.shellTool = nil
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
