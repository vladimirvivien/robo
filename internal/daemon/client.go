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
	"github.com/vladimirvivien/robo/internal/ui"
)

// Client coordinates sending requests to the local background daemon with fallback.
type Client struct {
	cfg            config.Config
	baseURL        string
	statePath      string
	httpClient     *http.Client
	inProcEngine   engine.Engine
	launcher       func() error
	cachedState    *State
	mu             sync.Mutex
	inProcFallback bool
}

// ClientOption configures a Client instance.
type ClientOption func(*Client)

// WithBaseURL overrides the local loopback URL (used for unit testing with dynamic ports).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithStatePath overrides the location of robod.json.
func WithStatePath(path string) ClientOption {
	return func(c *Client) {
		c.statePath = path
	}
}

// WithInProcEngine sets the fallback on-device engine.
func WithInProcEngine(eng engine.Engine) ClientOption {
	return func(c *Client) {
		c.inProcEngine = eng
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

// NewClient creates a new daemon Client for local IPC.
func NewClient(cfg config.Config, opts ...ClientOption) *Client {
	c := &Client{
		cfg:       cfg,
		baseURL:   config.DefaultRobodURL,
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
	baseURL, err := c.resolveEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	if c.cachedState != nil {
		return c.cachedState, nil
	}
	return &State{
		URL: baseURL,
	}, nil
}

func (c *Client) resolveEndpoint(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. If robod disabled, fail fast to in-process fallback
	if !c.cfg.Robod.Enabled {
		return "", errors.New("robod: daemon disabled")
	}

	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = config.DefaultRobodURL
	}

	// 2. Check if local endpoint is already running
	if c.pingURL(ctx, baseURL) == nil {
		if c.cachedState == nil {
			if state, err := LoadState(c.statePath); err == nil {
				c.cachedState = state
			}
		}
		return baseURL, nil
	}

	// 3. Try spawning local daemon
	ui.UpdateActiveSpinner("Launching daemon...")
	if err := c.launcher(); err != nil {
		return "", fmt.Errorf("robod: auto-spawn failed: %w", err)
	}

	ui.UpdateActiveSpinner("Loading model...")
	// Poll for readiness up to 10 seconds (gives model weights and GPU time to initialize)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		if c.pingURL(ctx, baseURL) == nil {
			if state, err := LoadState(c.statePath); err == nil {
				c.cachedState = state
			}
			return baseURL, nil
		}
	}

	return "", errors.New("daemon: timed out waiting for daemon readiness")
}

// Generate executes a synchronous completion against the daemon or in-proc fallback.
func (c *Client) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	baseURL, err := c.resolveEndpoint(ctx)
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

	url := fmt.Sprintf("%s/v1/generate", baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		ToolCalls:  genResp.ToolCalls,
		Provider:   genResp.Provider,
		Model:      genResp.Model,
		UsedLocal:  genResp.UsedLocal,
		TokensUsed: genResp.TokensUsed,
	}, nil
}

// GenerateStream yields tokens over a channel from the daemon's SSE stream.
func (c *Client) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	baseURL, err := c.resolveEndpoint(ctx)
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

	url := fmt.Sprintf("%s/v1/generate/stream", baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
						ToolCalls:  payload.ToolCalls,
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

func (c *Client) pingURL(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return errors.New("daemon: empty base URL")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("%s/health", strings.TrimRight(baseURL, "/"))
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

	cmd := exec.Command(exe, "daemon", "start", "--foreground")
	DetachCmd(cmd)
	return cmd.Start()
}

// StopDaemon attempts to shut down any active robod daemon instance gracefully.
// Returns true if a running daemon instance was found and shut down.
func StopDaemon(ctx context.Context, customStatePath ...string) (bool, error) {
	statePath := StatePath()
	if len(customStatePath) > 0 && customStatePath[0] != "" {
		statePath = customStatePath[0]
	}

	state, err := LoadState(statePath)
	if err != nil {
		return tryShutdownURL(ctx, config.DefaultRobodURL, statePath)
	}

	return tryShutdownURL(ctx, state.URL, statePath)
}

func tryShutdownURL(ctx context.Context, baseURL, statePath string) (bool, error) {
	if baseURL == "" {
		baseURL = config.DefaultRobodURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	shutdownURL := baseURL + "/v1/shutdown"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, shutdownURL, nil)
	if err != nil {
		_ = RemoveState(statePath)
		return false, nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_ = RemoveState(statePath)
		return false, nil
	}
	_ = resp.Body.Close()
	_ = RemoveState(statePath)

	// Wait briefly for daemon socket to terminate
	healthURL := baseURL + "/health"
	for range 10 {
		time.Sleep(50 * time.Millisecond)
		checkResp, checkErr := client.Get(healthURL)
		if checkErr != nil {
			break
		}
		_ = checkResp.Body.Close()
	}

	return true, nil
}
