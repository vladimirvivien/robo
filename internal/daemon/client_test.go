package daemon_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
)

func TestClient_DirectAndStream(t *testing.T) {
	mockEngine := engine.NewMockEngine("local-test")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon.json")

	token := "auth-tok-xyz"
	server, err := daemon.NewServer(mockEngine, daemon.ServerOptions{
		URL:       "http://127.0.0.1:0",
		AuthToken: token,
		ModelName: "test-model",
		StatePath: statePath,
		IdleTTL:   5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := server.Listen("http://127.0.0.1:0"); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx := t.Context()

	go func() { _ = server.Serve(ctx) }()

	cfg := *config.NewDefaultConfig()
	cfg.Robod.Enabled = true
	cfg.Robod.URL = server.URL()

	client := daemon.NewClient(cfg,
		daemon.WithStatePath(statePath),
		daemon.WithLauncher(func() error { return nil }),
	)

	// 1. Test Generate
	resp, err := client.Generate(ctx, engine.Request{Prompt: "hello from client"})
	if err != nil {
		t.Fatalf("client.Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "hello from client") {
		t.Errorf("unexpected response: %s", resp.Text)
	}

	// 2. Test GenerateStream
	streamCh, err := client.GenerateStream(ctx, engine.Request{Prompt: "stream word sequence"})
	if err != nil {
		t.Fatalf("client.GenerateStream failed: %v", err)
	}

	var gathered []string
	for chunk := range streamCh {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.Text != "" {
			gathered = append(gathered, chunk.Text)
		}
	}

	joined := strings.Join(gathered, "")
	if !strings.Contains(joined, "stream") || !strings.Contains(joined, "word") {
		t.Errorf("streamed text mismatch: %q", joined)
	}

	_ = server.Shutdown(ctx)
}

func TestClient_InProcessFallbackWhenDaemonDown(t *testing.T) {
	fallbackEngine := engine.NewMockEngine("in-proc-fallback")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "nonexistent-robod.json")

	cfg := *config.NewDefaultConfig()
	cfg.Robod.Enabled = false                // disable daemon to force immediate fallback
	cfg.Robod.URL = "http://127.0.0.1:59999" // ensure no background process is intercepted

	client := daemon.NewClient(cfg,
		daemon.WithStatePath(statePath),
		daemon.WithInProcEngine(fallbackEngine),
		daemon.WithLauncher(func() error { return nil }),
	)

	ctx := context.Background()

	// 1. Unary fallback
	resp, err := client.Generate(ctx, engine.Request{Prompt: "test fallback"})
	if err != nil {
		t.Fatalf("fallback Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "in-proc-fallback") {
		t.Errorf("expected response from fallback engine, got: %s", resp.Text)
	}

	// 2. Streaming fallback
	streamCh, err := client.GenerateStream(ctx, engine.Request{Prompt: "test streaming fallback"})
	if err != nil {
		t.Fatalf("fallback GenerateStream failed: %v", err)
	}

	var gathered []string
	for chunk := range streamCh {
		if chunk.Error != nil {
			t.Fatalf("fallback chunk error: %v", chunk.Error)
		}
		if chunk.Text != "" {
			gathered = append(gathered, chunk.Text)
		}
	}

	joined := strings.Join(gathered, "")
	if !strings.Contains(joined, "streaming") {
		t.Errorf("expected streamed response from fallback engine, got: %s", joined)
	}
}

func TestClient_ExplicitRemoteURL(t *testing.T) {
	mockEngine := engine.NewMockEngine("remote-gpu-box")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "remote-robod.json")

	token := "remote-token-secret"
	server, err := daemon.NewServer(mockEngine, daemon.ServerOptions{
		URL:       "http://127.0.0.1:0",
		AuthToken: token,
		ModelName: "remote-gemma",
		StatePath: statePath,
		IdleTTL:   5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := server.Listen("http://127.0.0.1:0"); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx := t.Context()
	go func() { _ = server.Serve(ctx) }()

	remoteURL := server.URL()

	cfg := *config.NewDefaultConfig()
	cfg.Robod.URL = remoteURL
	cfg.Robod.AuthToken = token
	cfg.Robod.AutoSpawn = false

	client := daemon.NewClient(cfg,
		daemon.WithLauncher(func() error {
			t.Fatal("launcher should NOT be called for remote URL")
			return nil
		}),
	)

	resp, err := client.Generate(ctx, engine.Request{Prompt: "remote test prompt"})
	if err != nil {
		t.Fatalf("remote Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "remote test prompt") {
		t.Errorf("unexpected response from remote daemon: %s", resp.Text)
	}

	_ = server.Shutdown(ctx)
}

func TestClient_TLSServerAndClient(t *testing.T) {
	mockEngine := engine.NewMockEngine("tls-gpu-box")

	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)
	statePath := filepath.Join(dir, "tls-robod.json")

	server, err := daemon.NewServer(mockEngine, daemon.ServerOptions{
		URL:       "https://127.0.0.1:0",
		ModelName: "tls-gemma",
		StatePath: statePath,
		IdleTTL:   5 * time.Minute,
		TLSCert:   certPath,
		TLSKey:    keyPath,
	})
	if err != nil {
		t.Fatalf("NewServer TLS: %v", err)
	}

	if err := server.Listen("https://127.0.0.1:0"); err != nil {
		t.Fatalf("Listen TLS: %v", err)
	}

	ctx := t.Context()
	go func() { _ = server.Serve(ctx) }()

	httpsURL := server.URL()
	if !strings.HasPrefix(httpsURL, "https://") {
		t.Fatalf("expected https:// URL, got %s", httpsURL)
	}

	// 1. Client with CA certificate
	cfg := *config.NewDefaultConfig()
	cfg.Robod.URL = httpsURL
	cfg.Robod.AutoSpawn = false
	cfg.Robod.TLS.CAFile = certPath

	client := daemon.NewClient(cfg,
		daemon.WithLauncher(func() error {
			t.Fatal("launcher should not be called")
			return nil
		}),
	)

	resp, err := client.Generate(ctx, engine.Request{Prompt: "hello secure world"})
	if err != nil {
		t.Fatalf("HTTPS Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "hello secure world") {
		t.Errorf("unexpected HTTPS response: %s", resp.Text)
	}

	_ = server.Shutdown(ctx)
}

func generateTestCert(t *testing.T, dir string) (string, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Robo Test"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("x509.CreateCertificate failed: %v", err)
	}

	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file failed: %v", err)
	}
	defer func() { _ = certOut.Close() }()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatalf("pem.Encode cert failed: %v", err)
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file failed: %v", err)
	}
	defer func() { _ = keyOut.Close() }()
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("pem.Encode key failed: %v", err)
	}

	return certPath, keyPath
}
