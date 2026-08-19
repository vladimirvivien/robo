package daemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

// ClientOption configures a Client instance.
type ClientOption func(*Client)

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

// NewClient creates a new daemon Client with optional TLS support.
func NewClient(cfg config.Config, opts ...ClientOption) *Client {
	transport := &http.Transport{}
	if cfg.Robod.TLS != nil {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Robod.TLS.InsecureSkipVerify, //nolint:gosec
		}

		if cfg.Robod.TLS.CAFile != "" {
			caData, err := os.ReadFile(cfg.Robod.TLS.CAFile)
			if err == nil {
				pool := x509.NewCertPool()
				if pool.AppendCertsFromPEM(caData) {
					tlsConfig.RootCAs = pool
				}
			}
		}
		transport.TLSClientConfig = tlsConfig
	}

	c := &Client{
		cfg:       cfg,
		statePath: StatePath(),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
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

type endpointTarget struct {
	baseURL string
	token   string
}

// EnsureDaemon ensures the background daemon is healthy, spawning it if necessary.
func (c *Client) EnsureDaemon(ctx context.Context) (*State, error) {
	target, err := c.resolveEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	if c.cachedState != nil {
		return c.cachedState, nil
	}
	return &State{
		URL:       target.baseURL,
		AuthToken: target.token,
	}, nil
}

func (c *Client) resolveEndpoint(ctx context.Context) (*endpointTarget, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	targetURL := c.cfg.Robod.URL
	if targetURL == "" {
		targetURL = config.DefaultRobodURL
	}
	baseURL := strings.TrimRight(targetURL, "/")
	token := c.cfg.Robod.AuthToken

	// 1. If remote endpoint (not localhost), connect directly
	if !isLocalEndpoint(baseURL) {
		return &endpointTarget{
			baseURL: baseURL,
			token:   token,
		}, nil
	}

	// 2. Check if local endpoint is already running
	if c.pingURL(ctx, baseURL) == nil {
		// Read token from state file if none configured
		if token == "" {
			if state, err := LoadState(c.statePath); err == nil {
				token = state.AuthToken
				c.cachedState = state
			}
		}
		return &endpointTarget{
			baseURL: baseURL,
			token:   token,
		}, nil
	}

	// 3. If robod disabled or auto-spawn disabled, fail fast to in-process fallback
	if !c.cfg.Robod.Enabled || !c.cfg.Robod.AutoSpawn {
		return nil, errors.New("robod: auto-spawn disabled or daemon inactive")
	}

	// 4. Try spawning local daemon
	if err := c.launcher(); err != nil {
		return nil, fmt.Errorf("robod: auto-spawn failed: %w", err)
	}

	// Poll for readiness up to 1 second
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}

		if c.pingURL(ctx, baseURL) == nil {
			if token == "" {
				if state, err := LoadState(c.statePath); err == nil {
					token = state.AuthToken
					c.cachedState = state
				}
			}
			return &endpointTarget{
				baseURL: baseURL,
				token:   token,
			}, nil
		}
	}

	return nil, errors.New("daemon: timed out waiting for daemon readiness")
}

func isLocalEndpoint(raw string) bool {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

// Generate executes a synchronous completion against the daemon or in-proc fallback.
func (c *Client) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	target, err := c.resolveEndpoint(ctx)
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

	url := fmt.Sprintf("%s/v1/generate", target.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if target.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+target.token)
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
	target, err := c.resolveEndpoint(ctx)
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

	url := fmt.Sprintf("%s/v1/generate/stream", target.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon client: new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if target.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+target.token)
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

	cmd := exec.Command(exe, "daemon", "start")
	return cmd.Start()
}
