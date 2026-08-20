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

		outMode := strings.ToLower(strings.TrimSpace(flagOutput))
		isInteractive := ui.IsStdoutTerminal() && (outMode == "markdown" || outMode == "md" || outMode == "")
		var sp *ui.Spinner
		if isInteractive {
			sp = ui.StartSpinner("Starting background robod daemon...")
			defer sp.Stop()
		}

		subCmd := exec.Command(executable, "daemon", "start", "--foreground")
		if cfgFile != "" {
			subCmd.Args = append(subCmd.Args, "--config", cfgFile)
		}

		if err := subCmd.Start(); err != nil {
			return fmt.Errorf("start daemon process: %w", err)
		}

		// Wait for daemon to become healthy
		client := daemon.NewClient(*cfg)
		_, err = client.EnsureDaemon(ctx)

		if sp != nil {
			sp.Stop()
		}

		if err != nil {
			return fmt.Errorf("daemon failed to reach ready state: %w", err)
		}

		if isInteractive {
			fmt.Println(ui.BadgeSuccess(fmt.Sprintf("✓ robod daemon started (PID: %d)", subCmd.Process.Pid)))
		} else {
			fmt.Printf("robod daemon started (PID: %d)\n", subCmd.Process.Pid)
		}
		return nil
	}

	// Foreground mode: start server directly
	eng := local.New(cfg.LLM.Local, cfg)

	opts := daemon.ServerOptions{
		URL:       cfg.Robod.URL,
		IdleTTL:   cfg.Robod.IdleTTL,
		AuthToken: cfg.Robod.AuthToken,
		ModelName: cfg.LLM.Local.Model,
	}
	if cfg.Robod.TLS != nil {
		opts.TLSCert = cfg.Robod.TLS.CertFile
		opts.TLSKey = cfg.Robod.TLS.KeyFile
	}

	server, err := daemon.NewServer(eng, opts)
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
	outMode := strings.ToLower(strings.TrimSpace(flagOutput))
	isInteractive := ui.IsStdoutTerminal() && (outMode == "markdown" || outMode == "md" || outMode == "")

	statePath := daemon.StatePath()
	state, err := daemon.LoadState(statePath)
	if err != nil {
		if outMode == "json" {
			fmt.Println(`{"status": "stopped", "running": false}`)
			return nil
		}
		if isInteractive {
			fmt.Println(ui.Card("robod Status", "Daemon is currently inactive (stopped).", "Run 'robo daemon start' to launch"))
			return nil
		}
		fmt.Println("robod is currently stopped")
		return nil
	}

	// Verify health
	healthURL := fmt.Sprintf("%s/health", state.URL)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		_ = daemon.RemoveState(statePath)
		if outMode == "json" {
			fmt.Println(`{"status": "unreachable", "running": false}`)
			return nil
		}
		if isInteractive {
			fmt.Println(ui.ErrorCard(fmt.Sprintf("robod state found at %s (PID %d), but server is unreachable.\nCleaning up stale state.", state.URL, state.PID)))
			return nil
		}
		fmt.Fprintf(os.Stderr, "robod at %s (PID %d) is unreachable. Cleaned up stale state.\n", state.URL, state.PID)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	uptime := time.Since(state.StartedAt).Truncate(time.Second)
	if outMode == "json" {
		statusData := map[string]any{
			"status":     "running",
			"running":    true,
			"url":        state.URL,
			"pid":        state.PID,
			"model":      state.Model,
			"uptime_sec": int(uptime.Seconds()),
		}
		data, _ := json.MarshalIndent(statusData, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if isInteractive {
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

	fmt.Printf("robod is running at %s (PID: %d, Model: %s, Uptime: %s)\n",
		state.URL, state.PID, state.Model, uptime)
	return nil
}
