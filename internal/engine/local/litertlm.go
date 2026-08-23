package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vladimirvivien/litertlm-go/pkg/litertlm"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/shell"
	"github.com/vladimirvivien/robo/internal/ui"
)

type toolCaptureCtxKey struct{}

type toolRecorder struct {
	mu    sync.Mutex
	calls []engine.ToolCall
}

func (r *toolRecorder) add(tc engine.ToolCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, tc)
}

func (r *toolRecorder) get() []engine.ToolCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return nil
	}
	copied := make([]engine.ToolCall, len(r.calls))
	copy(copied, r.calls)
	return copied
}

// Engine implements engine.Engine using Google LiteRT-LM.
type Engine struct {
	cfg       config.SLMConfig
	fullCfg   *config.Config
	client    *litertlm.Client
	shellTool *litertlm.ManagedTool[shell.ShellInput, shell.ShellOutput]
	mu        sync.Mutex
}

// New creates a new LiteRT-LM local engine instance.
func New(cfg config.SLMConfig, fullCfg ...*config.Config) *Engine {
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
	cacheDir := e.cfg.CacheDir
	if cacheDir == "" && modelPath != "" {
		cacheDir = filepath.Dir(modelPath)
	}
	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0750); err != nil {
			return nil, fmt.Errorf("local: create cache dir: %w", err)
		}
		opts = append(opts, litertlm.WithCacheDir(cacheDir))
	}

	ui.UpdateActiveSpinner("Loading model...")
	client, err := litertlm.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("local: initialize litertlm: %w", err)
	}

	shellTool, err := litertlm.RegisterTool(client, "execute_shell",
		"Propose a shell command or script to execute on the host operating system.",
		func(ctx context.Context, in shell.ShellInput) (shell.ShellOutput, error) {
			// In Two-Tier architecture, the inference tier evaluates and rates safety,
			// but NEVER executes commands against the host OS.
			assessment := shell.AssessSafety(in.Command)
			tc := engine.ToolCall{
				Name:          "execute_shell",
				Command:       in.Command,
				Description:   in.Description,
				Risk:          assessment.Level,
				Warning:       assessment.Warning,
				IsDestructive: assessment.IsDestructive,
			}
			if rec, ok := ctx.Value(toolCaptureCtxKey{}).(*toolRecorder); ok && rec != nil {
				rec.add(tc)
			}
			return shell.ShellOutput{
				Output: fmt.Sprintf("Command proposed for client execution: %s [Risk: %s]", in.Command, assessment.Level),
			}, nil
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
	chatOpts = append(chatOpts, litertlm.WithMaxToolHops(1))
	if req.SystemPrompt != "" {
		chatOpts = append(chatOpts, litertlm.WithSystemPrompt(req.SystemPrompt))
	}
	if e.shellTool != nil {
		chatOpts = append(chatOpts, litertlm.WithTool(e.shellTool))
	}

	rec := &toolRecorder{}
	execCtx := context.WithValue(ctx, toolCaptureCtxKey{}, rec)

	chat, err := client.NewChat(execCtx, chatOpts...)
	if err != nil {
		return nil, fmt.Errorf("local: new chat: %w", err)
	}
	defer func() { _ = chat.Close() }()

	var runtimeOpts []litertlm.RuntimeOption
	if req.MaxTokens > 0 {
		runtimeOpts = append(runtimeOpts, litertlm.WithMaxOutputTokens(req.MaxTokens))
	}

	reply, err := chat.Send(execCtx, fullPrompt, runtimeOpts...)
	if err != nil {
		if errors.Is(err, litertlm.ErrToolHopsExceeded) && len(rec.get()) > 0 {
			return &engine.Response{
				Text:       "",
				ToolCalls:  rec.get(),
				Provider:   "litertlm",
				Model:      e.cfg.Model,
				UsedLocal:  true,
				TokensUsed: 0,
			}, nil
		}
		return nil, fmt.Errorf("local: chat send: %w", err)
	}

	return &engine.Response{
		Text:       reply.Text(),
		ToolCalls:  rec.get(),
		Provider:   "litertlm",
		Model:      e.cfg.Model,
		UsedLocal:  true,
		TokensUsed: 0,
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
		chatOpts = append(chatOpts, litertlm.WithMaxToolHops(1))
		if req.SystemPrompt != "" {
			chatOpts = append(chatOpts, litertlm.WithSystemPrompt(req.SystemPrompt))
		}
		if e.shellTool != nil {
			chatOpts = append(chatOpts, litertlm.WithTool(e.shellTool))
		}

		rec := &toolRecorder{}
		execCtx := context.WithValue(ctx, toolCaptureCtxKey{}, rec)

		chat, err := client.NewChat(execCtx, chatOpts...)
		if err != nil {
			out <- engine.StreamChunk{Error: fmt.Errorf("local: new chat: %w", err)}
			return
		}
		defer func() { _ = chat.Close() }()

		for chunk, err := range chat.SendStream(execCtx, fullPrompt, runtimeOpts...) {
			if err != nil {
				if errors.Is(err, litertlm.ErrToolHopsExceeded) && len(rec.get()) > 0 {
					out <- engine.StreamChunk{
						ToolCalls: rec.get(),
						Final:     true,
					}
					return
				}
				out <- engine.StreamChunk{Error: err}
				return
			}
			out <- engine.StreamChunk{
				Text:      chunk.Text,
				ToolCalls: rec.get(),
				Final:     chunk.Final,
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
