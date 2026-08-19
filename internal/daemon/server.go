package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vladimirvivien/robo/internal/engine"
)

// Server is the HTTP server hosting warm on-device model execution.
type Server struct {
	engine     engine.Engine
	listener   net.Listener
	httpServer *http.Server
	watchdog   *Watchdog
	authToken  string
	modelName  string
	statePath  string
	tlsCert    string
	tlsKey     string
	mu         sync.Mutex
	running    bool
	startedAt  time.Time
}

// ServerOptions configures the daemon server.
type ServerOptions struct {
	URL       string // Unified URL e.g. "http://127.0.0.1:8765" or "https://0.0.0.0:8765"
	IdleTTL   time.Duration
	AuthToken string
	ModelName string
	StatePath string
	TLSCert   string // Path to TLS certificate for HTTPS
	TLSKey    string // Path to TLS private key
}

// NewServer creates a new daemon Server with the given engine and options.
func NewServer(eng engine.Engine, opts ServerOptions) (*Server, error) {
	if eng == nil {
		return nil, errors.New("daemon: engine cannot be nil")
	}

	authToken := opts.AuthToken

	modelName := opts.ModelName
	if modelName == "" {
		modelName = "local-model"
	}

	s := &Server{
		engine:    eng,
		authToken: authToken,
		modelName: modelName,
		statePath: opts.StatePath,
		tlsCert:   opts.TLSCert,
		tlsKey:    opts.TLSKey,
		startedAt: time.Now(),
	}

	s.watchdog = NewWatchdog(opts.IdleTTL, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /v1/generate", RequireAuth(authToken, s.handleGenerate))
	mux.HandleFunc("POST /v1/generate/stream", RequireAuth(authToken, s.handleGenerateStream))
	mux.HandleFunc("POST /v1/shutdown", RequireAuth(authToken, s.handleShutdown))

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second,
	}

	return s, nil
}

// Listen starts listening on the network address specified by rawURL (e.g. "http://127.0.0.1:8765" or "http://127.0.0.1:0").
func (s *Server) Listen(rawURL string) error {
	if rawURL == "" {
		rawURL = "http://127.0.0.1:8765"
	}

	host, port, err := ParseURLHostPort(rawURL)
	if err != nil {
		return fmt.Errorf("daemon: parse url: %w", err)
	}

	addr := net.JoinHostPort(host, port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("daemon: listen %s: %w", addr, err)
	}
	s.listener = l
	return nil
}

// Addr returns the resolved network address of the listener.
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Port returns the actual bound TCP port.
func (s *Server) Port() int {
	if s.listener != nil {
		if tcpAddr, ok := s.listener.Addr().(*net.TCPAddr); ok {
			return tcpAddr.Port
		}
	}
	return 0
}

// URL returns the effective HTTP or HTTPS URL of the bound server.
func (s *Server) URL() string {
	if s.listener != nil {
		if tcpAddr, ok := s.listener.Addr().(*net.TCPAddr); ok {
			host := tcpAddr.IP.String()
			if host == "::" || host == "0.0.0.0" {
				host = "127.0.0.1"
			}
			scheme := "http"
			if s.tlsCert != "" && s.tlsKey != "" {
				scheme = "https"
			}
			return fmt.Sprintf("%s://%s:%d", scheme, host, tcpAddr.Port)
		}
	}
	return ""
}

// AuthToken returns the secret bearer token for this daemon instance.
func (s *Server) AuthToken() string {
	return s.authToken
}

// Serve runs the HTTP/HTTPS server and starts the watchdog timer.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	if s.listener == nil {
		s.mu.Unlock()
		return errors.New("daemon: server not listening; call Listen first")
	}
	s.running = true
	s.mu.Unlock()

	// Write robod.json state file
	state := State{
		URL:       s.URL(),
		Port:      s.Port(),
		PID:       os.Getpid(),
		AuthToken: s.authToken,
		Model:     s.modelName,
		StartedAt: s.startedAt,
		LastTouch: time.Now(),
	}
	if err := SaveState(s.statePath, state); err != nil {
		return fmt.Errorf("daemon: save state: %w", err)
	}

	// Start watchdog loop in background
	go s.watchdog.Start(ctx)

	var err error
	if s.tlsCert != "" && s.tlsKey != "" {
		err = s.httpServer.ServeTLS(s.listener, s.tlsCert, s.tlsKey)
	} else {
		err = s.httpServer.Serve(s.listener)
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ParseURLHostPort extracts host and port from a URL or host:port string.
func ParseURLHostPort(raw string) (string, string, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := u.Port()
	if port == "" {
		port = "8765"
	}
	return host, port, nil
}

// Shutdown cleanly stops the server, cleans up state, and closes the engine.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	s.watchdog.Stop()
	_ = RemoveState(s.statePath)

	var errs []error
	if err := s.httpServer.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := s.engine.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("daemon: shutdown errors: %v", errs)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := HealthResponse{
		Status: "ok",
		Model:  s.modelName,
		PID:    os.Getpid(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	s.watchdog.Touch()

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bad request: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	engineReq := engine.Request{
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		ContextFiles: req.ContextFiles,
		Images:       req.Images,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	}

	resp, err := s.engine.Generate(r.Context(), engineReq)
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(GenerateResponse{Error: err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(GenerateResponse{
		Text:       resp.Text,
		Provider:   resp.Provider,
		Model:      resp.Model,
		UsedLocal:  resp.UsedLocal,
		TokensUsed: resp.TokensUsed,
	})
}

func (s *Server) handleGenerateStream(w http.ResponseWriter, r *http.Request) {
	s.watchdog.Touch()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bad request: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	engineReq := engine.Request{
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		ContextFiles: req.ContextFiles,
		Images:       req.Images,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	}

	chunkCh, err := s.engine.GenerateStream(r.Context(), engineReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"stream start failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	for chunk := range chunkCh {
		s.watchdog.Touch()
		payload := StreamChunkPayload{
			Text:       chunk.Text,
			Final:      chunk.Final,
			TokensUsed: chunk.TokensUsed,
		}
		if chunk.Error != nil {
			payload.Error = chunk.Error.Error()
		}

		data, err := json.Marshal(payload)
		if err == nil {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	go func() {
		time.Sleep(100 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
}
