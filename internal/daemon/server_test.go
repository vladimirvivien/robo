package daemon_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
)

func TestServer_HealthAuthGenerateStream(t *testing.T) {
	mock := engine.NewMockEngine("local-mock")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon.json")

	token := "test-secret-token-123"
	server, err := daemon.NewServer(mock, daemon.ServerOptions{
		URL:       "http://127.0.0.1:0", // dynamic port
		AuthToken: token,
		ModelName: "test-gemma",
		StatePath: statePath,
		IdleTTL:   10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if err := server.Listen("http://127.0.0.1:0"); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	ctx := t.Context()

	go func() {
		_ = server.Serve(ctx)
	}()

	baseURL := server.URL()
	if baseURL == "" {
		t.Fatal("expected non-empty server URL")
	}

	// 1. Test GET /health (unauthenticated)
	healthResp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer func() { _ = healthResp.Body.Close() }()

	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("health status: expected 200, got %d", healthResp.StatusCode)
	}
	var health daemon.HealthResponse
	_ = json.NewDecoder(healthResp.Body).Decode(&health)
	if health.Model != "test-gemma" {
		t.Errorf("health model: expected test-gemma, got %s", health.Model)
	}

	// 2. Test POST /v1/generate without token (expect 401)
	reqBody := `{"prompt":"ping"}`
	unauthResp, err := http.Post(baseURL+"/v1/generate", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST unauth failed: %v", err)
	}
	_ = unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthorized, got %d", unauthResp.StatusCode)
	}

	// 3. Test POST /v1/generate with valid token
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/generate", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	genResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/generate failed: %v", err)
	}
	defer func() { _ = genResp.Body.Close() }()

	if genResp.StatusCode != http.StatusOK {
		t.Errorf("generate status: expected 200, got %d", genResp.StatusCode)
	}
	var res daemon.GenerateResponse
	_ = json.NewDecoder(genResp.Body).Decode(&res)
	if !strings.Contains(res.Text, "ping") {
		t.Errorf("unexpected response text: %s", res.Text)
	}

	// 4. Test POST /v1/generate/stream (SSE)
	streamReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/generate/stream", strings.NewReader(`{"prompt":"streaming test message"}`))
	if err != nil {
		t.Fatalf("new stream req: %v", err)
	}
	streamReq.Header.Set("Authorization", "Bearer "+token)
	streamReq.Header.Set("Content-Type", "application/json")

	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("POST /v1/generate/stream failed: %v", err)
	}
	defer func() { _ = streamResp.Body.Close() }()

	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("stream status: expected 200, got %d", streamResp.StatusCode)
	}

	var chunks []string
	scanner := bufio.NewScanner(streamResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data := after
			var p daemon.StreamChunkPayload
			if err := json.Unmarshal([]byte(data), &p); err == nil && p.Text != "" {
				chunks = append(chunks, p.Text)
			}
		}
	}

	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "streaming") || !strings.Contains(joined, "message") {
		t.Errorf("streamed output mismatch, got: %q", joined)
	}

	// Clean shutdown
	_ = server.Shutdown(ctx)
}

func TestServer_RequireAuthMiddleware(t *testing.T) {
	token := "secure-token"
	handler := daemon.RequireAuth(token, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Test missing header
	req, _ := http.NewRequest("GET", "/", nil)
	w := &stubResponseWriter{header: make(http.Header)}
	handler(w, req)
	if w.statusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing auth, got %d", w.statusCode)
	}

	// Test valid header
	req.Header.Set("Authorization", "Bearer "+token)
	w = &stubResponseWriter{header: make(http.Header)}
	handler(w, req)
	if w.statusCode != http.StatusOK {
		t.Errorf("expected 200 on valid auth, got %d", w.statusCode)
	}
}

type stubResponseWriter struct {
	header     http.Header
	statusCode int
	buf        bytes.Buffer
}

func (s *stubResponseWriter) Header() http.Header {
	return s.header
}

func (s *stubResponseWriter) Write(b []byte) (int, error) {
	if s.statusCode == 0 {
		s.statusCode = http.StatusOK
	}
	return s.buf.Write(b)
}

func (s *stubResponseWriter) WriteHeader(statusCode int) {
	s.statusCode = statusCode
}
