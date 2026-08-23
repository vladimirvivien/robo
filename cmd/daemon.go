package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

		daemon.DetachCmd(subCmd)
		if err := subCmd.Start(); err != nil {
			return fmt.Errorf("start daemon process: %w", err)
		}

		// Wait for daemon to become healthy
		client := daemon.NewClient(*cfg, daemon.WithLauncher(func() error { return nil }))
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
	eng := local.New(cfg.SLM, cfg)

	opts := daemon.ServerOptions{
		URL:       config.DefaultRobodURL,
		IdleTTL:   cfg.Robod.IdleTTL,
		ModelName: cfg.SLM.Model,
	}

	server, err := daemon.NewServer(eng, opts)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if err := server.Listen(config.DefaultRobodURL); err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("robod daemon running on %s (PID: %d, Idle TTL: %s)\n", server.URL(), os.Getpid(), cfg.Robod.IdleTTL)
	serveErr := server.Serve(ctx)
	if serveErr != nil {
		fmt.Printf("robod daemon Serve returned error: %v\n", serveErr)
	} else {
		fmt.Println("robod daemon Serve exited cleanly")
	}
	return serveErr
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	state, err := daemon.LoadState(daemon.StatePath())
	shutdownURL := config.DefaultRobodURL + "/v1/shutdown"
	var pid int

	if err == nil {
		shutdownURL = fmt.Sprintf("%s/v1/shutdown", state.URL)
		pid = state.PID
	}

	// Send shutdown POST request
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, shutdownURL, nil)
	if err != nil {
		fmt.Println("robod is not running")
		return nil
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_ = daemon.RemoveState(daemon.StatePath())
		fmt.Println("robod is not running")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	_ = daemon.RemoveState(daemon.StatePath())

	// Wait briefly for daemon socket to terminate
	healthURL := config.DefaultRobodURL + "/health"
	for range 10 {
		time.Sleep(50 * time.Millisecond)
		checkResp, checkErr := client.Get(healthURL)
		if checkErr != nil {
			break
		}
		_ = checkResp.Body.Close()
	}

	if pid > 0 {
		fmt.Printf("robod daemon stopped (PID: %d)\n", pid)
	} else {
		fmt.Println("robod daemon stopped")
	}
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	outMode := strings.ToLower(strings.TrimSpace(flagOutput))

	statePath := daemon.StatePath()
	state, err := daemon.LoadState(statePath)
	healthURL := config.DefaultRobodURL + "/health"
	var pid int
	var model string

	if err == nil {
		healthURL = fmt.Sprintf("%s/health", state.URL)
		pid = state.PID
		model = state.Model
	} else {
		cfg, _ := config.Load(cfgFile)
		if cfg != nil {
			model = cfg.SLM.Model
		}
	}

	// Verify health
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		_ = daemon.RemoveState(statePath)
		if outMode == "json" {
			fmt.Println(`{"status": "stopped", "running": false}`)
			return nil
		}
		fmt.Println("robod is stopped")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if outMode == "json" {
		statusData := map[string]any{
			"status":  "running",
			"running": true,
		}
		if pid > 0 {
			statusData["pid"] = pid
		}
		if model != "" {
			statusData["model"] = model
		}
		data, _ := json.MarshalIndent(statusData, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if pid > 0 && model != "" {
		fmt.Printf("robod is running (PID: %d, Model: %s)\n", pid, model)
	} else if pid > 0 {
		fmt.Printf("robod is running (PID: %d)\n", pid)
	} else {
		fmt.Println("robod is running")
	}
	return nil
}
