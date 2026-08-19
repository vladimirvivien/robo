package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine/local"
	"github.com/vladimirvivien/robo/internal/ui"
)

// DaemonCmd represents the daemon management command tree.
var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background robod daemon server",
	Long:  `Manage the robod hot-start background daemon hosting warm on-device LiteRT-LM models.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the robod daemon server",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running robod daemon server",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the background robod daemon",
	RunE:  runDaemonStatus,
}

var flagForeground bool

func init() {
	daemonStartCmd.Flags().BoolVarP(&flagForeground, "foreground", "f", false, "run daemon in foreground")

	DaemonCmd.AddCommand(daemonStartCmd)
	DaemonCmd.AddCommand(daemonStopCmd)
	DaemonCmd.AddCommand(daemonStatusCmd)
	RootCmd.AddCommand(DaemonCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !flagForeground {
		// Launch detached background process with visual feedback
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable: %w", err)
		}

		sp := ui.StartSpinner("Starting background robod daemon...")
		defer sp.Stop()

		subCmd := exec.Command(executable, "daemon", "start", "--foreground")
		if cfgFile != "" {
			subCmd.Args = append(subCmd.Args, "--config", cfgFile)
		}
		daemon.DetachCmd(subCmd)

		if err := subCmd.Start(); err != nil {
			return fmt.Errorf("spawn background robod: %w", err)
		}

		// Wait for daemon to become ready
		client := &http.Client{Timeout: 300 * time.Millisecond}
		healthURL := fmt.Sprintf("%s/health", strings.TrimRight(cfg.Robod.URL, "/"))
		if healthURL == "" {
			healthURL = "http://127.0.0.1:8765/health"
		}

		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			resp, err := client.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				ready = true
				break
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}

		sp.Stop()

		if ready {
			fmt.Printf("robod daemon is running at %s (PID: %d)\n", cfg.Robod.URL, subCmd.Process.Pid)
		} else {
			fmt.Printf("Started background robod daemon (PID: %d) - initializing...\n", subCmd.Process.Pid)
		}
		return nil
	}

	// Foreground execution
	eng := local.New(cfg.LLM.Local)
	defer func() { _ = eng.Close() }()

	var tlsCert, tlsKey string
	if cfg.Robod.TLS != nil {
		tlsCert = cfg.Robod.TLS.CertFile
		tlsKey = cfg.Robod.TLS.KeyFile
	}

	serverOpts := daemon.ServerOptions{
		URL:       cfg.Robod.URL,
		AuthToken: cfg.Robod.AuthToken,
		ModelName: cfg.LLM.Local.Model,
		StatePath: daemon.StatePath(),
		IdleTTL:   cfg.Robod.IdleTTL,
		TLSCert:   tlsCert,
		TLSKey:    tlsKey,
	}

	server, err := daemon.NewServer(eng, serverOpts)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if err := server.Listen(cfg.Robod.URL); err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("robod daemon running on %s (PID: %d, Idle TTL: %s)\n", server.URL(), os.Getpid(), cfg.Robod.IdleTTL)
	return server.Serve(ctx)
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	state, err := daemon.LoadState(daemon.StatePath())
	if err != nil {
		return fmt.Errorf("robod is not running (no active state file found)")
	}

	// Send shutdown POST request
	shutdownURL := fmt.Sprintf("%s/v1/shutdown", state.URL)
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, shutdownURL, nil)
	if err != nil {
		return err
	}
	if state.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", state.AuthToken))
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Clean up stale file if process is already gone
		_ = daemon.RemoveState(daemon.StatePath())
		fmt.Printf("Cleaned up stale robod state file (PID: %d)\n", state.PID)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Printf("Sent shutdown signal to robod daemon at %s (PID: %d)\n", state.URL, state.PID)
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	statePath := daemon.StatePath()
	state, err := daemon.LoadState(statePath)
	if err != nil {
		fmt.Println(ui.Card("robod Status", "Daemon is currently inactive (stopped).", "Run 'robo daemon start' to launch"))
		return nil
	}

	// Verify health
	healthURL := fmt.Sprintf("%s/health", state.URL)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Println(ui.ErrorCard(fmt.Sprintf("robod state found at %s (PID %d), but server is unreachable.\nCleaning up stale state.", state.URL, state.PID)))
		_ = daemon.RemoveState(statePath)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	var health map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&health)

	uptime := time.Since(state.StartedAt).Truncate(time.Second)
	content := fmt.Sprintf(
		"Status:   Active (Running)\nURL:      %s\nPID:      %d\nModel:    %s\nUptime:   %s\nState:    %s",
		state.URL,
		state.PID,
		state.Model,
		uptime,
		filepath.Base(statePath),
	)

	fmt.Println(ui.Card("robod Daemon Status", content, "Hot-start ready"))
	return nil
}
