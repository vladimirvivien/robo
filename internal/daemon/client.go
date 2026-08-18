package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

// Client coordinates sending requests to the background daemon with fallback.
type Client struct {
	cfg            config.Config
	statePath      string
	httpClient     *http.Client
	inProcEngine   engine.Engine
	launcher       func() error
	cachedState    *State
	mu             sync.Mutex
	inProcFallback bool
}

// ClientOption configures the daemon client.
type ClientOption func(*Client)

// WithInProcEngine configures an in-process fallback engine.
func WithInProcEngine(eng engine.Engine) ClientOption {
	return func(c *Client) {
		c.inProcEngine = eng
	}
}

// WithStatePath overrides the daemon state file path.
func WithStatePath(path string) ClientOption {
	return func(c *Client) {
		c.statePath = path
	}
}

// WithLauncher overrides the daemon background launcher function.
func WithLauncher(fn func() error) ClientOption {
	return func(c *Client) {
		c.launcher = fn
	}
}

// WithHTTPClient overrides the internal http.Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new daemon Client.
func NewClient(cfg config.Config, opts ...ClientOption) *Client {
	c := &Client{
		cfg:       cfg,
		statePath: StatePath(),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		inProcFallback: true,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.launcher == nil {
		c.launcher = c.defaultLauncher
	}

	return c
}

// Name returns the client identifier.
func (c *Client) Name() string {
	return "daemon-client"
}

// EnsureDaemon ensures the background daemon is healthy, spawning it if necessary.
func (c *Client) EnsureDaemon(ctx context.Context) (*State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if cached or on-disk state is currently healthy
	state, err := LoadState(c.statePath)
	if err == nil && c.ping(ctx, state) == nil {
		c.cachedState = state
		return state, nil
	}

	// If daemon disabled in config, fail fast to in-process fallback
	if !c.cfg.Daemon.Enabled {
		return nil, errors.New("daemon: disabled in configuration")
	}

	// Try spawning the daemon
	if err := c.launcher(); err != nil {
		return nil, fmt.Errorf("daemon: auto-spawn failed: %w", err)
	}

	// Poll for readiness up to 3 seconds
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		state, err := LoadState(c.statePath)
		if err == nil && c.ping(ctx, state) == nil {
			c.cachedState = state
			return state, nil
		}
	}

	return nil, errors.New("daemon: timed out waiting for daemon readiness")
}

// Generate executes a synchronous completion against the daemon or in-proc fallback.
func (c *Client) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	state, err := c.EnsureDaemon(ctx)
	if err != nil {
		if c.inProcEngine != nil && c.inProcFallback {
			return c.inProcEngine.Generate(ctx, req)
		}
		return nil, err
	}

	payload := GenerateRequest{
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		ContextFiles: req.ContextFiles,
		Images:       req.Images,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("daemon client: marshal request: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/generate", state.Port)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if state.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+state.AuthToken)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if c.inProcEngine != nil && c.inProcFallback {
			return c.inProcEngine.Generate(ctx, req)
		}
		return nil, fmt.Errorf("daemon client: post: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		var genErr GenerateResponse
		_ = json.NewDecoder(httpResp.Body).Decode(&genErr)
		if genErr.Error != "" {
			return nil, errors.New(genErr.Error)
		}
		return nil, fmt.Errorf("daemon client: unexpected status %d", httpResp.StatusCode)
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("daemon client: decode response: %w", err)
	}

	return &engine.Response{
		Text:       genResp.Text,
		Provider:   genResp.Provider,
		Model:      genResp.Model,
		UsedLocal:  genResp.UsedLocal,
		TokensUsed: genResp.TokensUsed,
	}, nil
}

// GenerateStream yields tokens over a channel from the daemon's SSE stream.
func (c *Client) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	state, err := c.EnsureDaemon(ctx)
	if err != nil {
		if c.inProcEngine != nil && c.inProcFallback {
			return c.inProcEngine.GenerateStream(ctx, req)
		}
		return nil, err
	}

	payload := GenerateRequest{
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		ContextFiles: req.ContextFiles,
		Images:       req.Images,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("daemon client: marshal request: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/generate/stream", state.Port)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if state.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+state.AuthToken)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if c.inProcEngine != nil && c.inProcFallback {
			return c.inProcEngine.GenerateStream(ctx, req)
		}
		return nil, fmt.Errorf("daemon client: stream request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		_ = httpResp.Body.Close()
		if c.inProcEngine != nil && c.inProcFallback {
			return c.inProcEngine.GenerateStream(ctx, req)
		}
		return nil, fmt.Errorf("daemon client: stream status %d", httpResp.StatusCode)
	}

	out := make(chan engine.StreamChunk, 20)

	go func() {
		defer func() { _ = httpResp.Body.Close() }()
		defer close(out)

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				data := after
				var payload StreamChunkPayload
				if err := json.Unmarshal([]byte(data), &payload); err == nil {
					chunk := engine.StreamChunk{
						Text:       payload.Text,
						Final:      payload.Final,
						TokensUsed: payload.TokensUsed,
					}
					if payload.Error != "" {
						chunk.Error = errors.New(payload.Error)
					}
					out <- chunk
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- engine.StreamChunk{Error: err}
		}
	}()

	return out, nil
}

// Close closes any underlying fallback engine.
func (c *Client) Close() error {
	if c.inProcEngine != nil {
		return c.inProcEngine.Close()
	}
	return nil
}

func (c *Client) ping(ctx context.Context, state *State) error {
	if state == nil || state.Port <= 0 {
		return errors.New("daemon: invalid state")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/health", state.Port)
	req, err := http.NewRequestWithContext(pingCtx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) defaultLauncher() error {
	exe, err := os.Executable()
	if err != nil {
		exe = "robo"
	}

	cmd := exec.Command(exe, "daemon", "start")
	return cmd.Start()
}
