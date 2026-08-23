package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/ui"
)

var flagStatusJSON bool

// StatusReport encapsulates the complete status of Robo configuration, models, and runtime.
type StatusReport struct {
	ConfigPath   string           `json:"config_path"`
	ConfigExists bool             `json:"config_exists"`
	Robo         RoboStatusInfo   `json:"robo"`
	SLM          SLMStatusInfo    `json:"slm"`
	LLM          LLMStatusInfo    `json:"llm"`
	Robod        DaemonStatusInfo `json:"robod"`
}

// RoboStatusInfo describes general application and inference mode settings.
type RoboStatusInfo struct {
	InferenceMode  string `json:"inference_mode"`
	OutputMode     string `json:"output_mode"`
	CaptureHistory bool   `json:"capture_history"`
}

// SLMStatusInfo describes the on-device LiteRT-LM model and runtime status.
type SLMStatusInfo struct {
	Model        string `json:"model"`
	Backend      string `json:"backend"`
	MaxTokens    int    `json:"max_tokens"`
	Version      string `json:"version"`
	CacheDir     string `json:"cache_dir,omitempty"`
	ModelPath    string `json:"model_path,omitempty"`
	ModelFound   bool   `json:"model_found"`
	LibraryFound bool   `json:"library_found"`
}

// LLMStatusInfo describes the frontier cloud model configuration and credentials.
type LLMStatusInfo struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
	Configured bool   `json:"configured"`
}

// DaemonStatusInfo describes the background robod process status.
type DaemonStatusInfo struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Model   string `json:"model,omitempty"`
	URL     string `json:"url,omitempty"`
	IdleTTL string `json:"idle_ttl,omitempty"`
}

// StatusCmd represents the "robo status" subcommand.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display current model configuration, runtime dependencies, and daemon status",
	Long: `Inspects resolved configuration, on-device SLM settings, runtime libraries,
remote LLM settings, and the background robod daemon process state.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runStatus,
}

func init() {
	StatusCmd.Flags().BoolVar(&flagStatusJSON, "json", false, "output status in JSON format")
	RootCmd.AddCommand(StatusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	targetConfigPath := cfgFile
	if targetConfigPath == "" {
		targetConfigPath = config.ConfigPath()
	}

	configExists := config.ConfigFileExists(targetConfigPath)
	var cfg *config.Config
	if configExists {
		loaded, err := config.Load(targetConfigPath)
		if err == nil {
			cfg = loaded
		}
	}
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	// Robo general status
	roboStatus := RoboStatusInfo{
		InferenceMode:  cfg.Robo.InferenceMode,
		OutputMode:     cfg.Robo.OutputMode,
		CaptureHistory: cfg.Robo.CaptureHistory,
	}

	// On-device SLM inspection
	localModelPath := engine.FindLocalModelPath(cfg.SLM.Model, cfg.SLM.CacheDir)
	localLibFound := engine.IsLibDownloaded(cfg.SLM.Version)
	slmStatus := SLMStatusInfo{
		Model:        cfg.SLM.Model,
		Backend:      cfg.SLM.Backend,
		MaxTokens:    cfg.SLM.MaxTokens,
		Version:      cfg.SLM.Version,
		CacheDir:     cfg.SLM.CacheDir,
		ModelPath:    localModelPath,
		ModelFound:   localModelPath != "",
		LibraryFound: localLibFound,
	}

	// Remote LLM inspection
	cloudCheck := engine.CheckCloudSetup(cfg.LLM)
	llmStatus := LLMStatusInfo{
		Provider:   cfg.LLM.Provider,
		Model:      cfg.LLM.Model,
		APIKeyEnv:  cloudCheck.APIKeyEnv,
		Configured: cloudCheck.Configured,
	}

	// Daemon status inspection
	statePath := daemon.StatePath()
	state, _ := daemon.LoadState(statePath)
	daemonRunning := false
	daemonPID := 0
	daemonModel := ""
	daemonURL := cfg.Robod.URL
	if daemonURL == "" {
		daemonURL = config.DefaultRobodURL
	}

	healthURL := daemonURL + "/health"
	if state != nil {
		healthURL = fmt.Sprintf("%s/health", state.URL)
		daemonPID = state.PID
		daemonModel = state.Model
		daemonURL = state.URL
	}

	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(healthURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		daemonRunning = true
		_ = resp.Body.Close()
	} else {
		_ = daemon.RemoveState(statePath)
		daemonPID = 0
		daemonModel = ""
	}

	daemonInfo := DaemonStatusInfo{
		Enabled: cfg.Robod.Enabled,
		Running: daemonRunning,
		PID:     daemonPID,
		Model:   daemonModel,
		URL:     daemonURL,
		IdleTTL: cfg.Robod.IdleTTL.String(),
	}

	report := StatusReport{
		ConfigPath:   targetConfigPath,
		ConfigExists: configExists,
		Robo:         roboStatus,
		SLM:          slmStatus,
		LLM:          llmStatus,
		Robod:        daemonInfo,
	}

	outMode := strings.ToLower(strings.TrimSpace(flagOutput))
	if flagStatusJSON || outMode == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Interactive Card output
	if ui.IsStdoutTerminal() && (outMode == "markdown" || outMode == "md" || outMode == "") {
		var sb strings.Builder

		// Config section
		if configExists {
			inferenceBadge := "(Active - Local SLM)"
			if cfg.Robo.InferenceMode == "llm" {
				inferenceBadge = "(Active - Remote LLM)"
			}
			fmt.Fprintf(&sb, "Config:\n  • File:           %s\n  • Inference Mode: %s %s\n  • Output Mode:    %s\n\n",
				targetConfigPath,
				cfg.Robo.InferenceMode,
				inferenceBadge,
				cfg.Robo.OutputMode,
			)
		} else {
			fmt.Fprintf(&sb, "Config:\n  • File:           %s (Not Found)\n\n", targetConfigPath)
			sb.WriteString("Status:\n  • robo is not initialized. Run 'robo init' to set up local models.\n")
			fmt.Println(ui.Card(
				ui.BadgeSuccess("🤖 robo • System & Model Status"),
				sb.String(),
				"",
			))
			return nil
		}

		// On-Device SLM section
		localModelStatus := "(Not Found - run 'robo init')"
		if slmStatus.ModelFound {
			localModelStatus = fmt.Sprintf("%s (Ready)", slmStatus.ModelPath)
		}

		localLibStatus := "(Missing - run 'robo init')"
		if slmStatus.LibraryFound {
			localLibStatus = fmt.Sprintf("LiteRT-LM %s (Installed)", slmStatus.Version)
		} else {
			localLibStatus = fmt.Sprintf("LiteRT-LM %s %s", slmStatus.Version, localLibStatus)
		}

		slmHeader := "On-Device SLM (LiteRT-LM):"
		if cfg.Robo.InferenceMode != "llm" {
			slmHeader = "On-Device SLM (LiteRT-LM • Active):"
		}

		fmt.Fprintf(&sb, "%s\n  • Model:          %s\n  • Backend:        %s\n  • Max Tokens:     %d\n  • Runtime:        %s\n  • Weights:        %s\n\n",
			slmHeader,
			slmStatus.Model,
			slmStatus.Backend,
			slmStatus.MaxTokens,
			localLibStatus,
			localModelStatus,
		)

		// Remote LLM section (only display if configured or explicitly selected)
		if llmStatus.Configured || cfg.Robo.InferenceMode == "llm" || cfg.LLM.IsConfigured() {
			cloudKeyStatus := fmt.Sprintf("%s (Not Set)", llmStatus.APIKeyEnv)
			if llmStatus.Configured {
				cloudKeyStatus = fmt.Sprintf("%s (Configured)", llmStatus.APIKeyEnv)
			}
			llmHeader := "Remote LLM:"
			if cfg.Robo.InferenceMode == "llm" {
				llmHeader = "Remote LLM (Active):"
			}
			fmt.Fprintf(&sb, "%s\n  • Provider:       %s\n  • Model:          %s\n  • API Key:        %s\n\n",
				llmHeader,
				llmStatus.Provider,
				llmStatus.Model,
				cloudKeyStatus,
			)
		}

		// Daemon section
		daemonText := "Stopped"
		if daemonInfo.Running {
			if daemonInfo.PID > 0 {
				daemonText = fmt.Sprintf("Running (PID: %d)", daemonInfo.PID)
			} else {
				daemonText = "Running"
			}
		}
		fmt.Fprintf(&sb, "Daemon (robod):\n  • State:          %s\n  • Idle TTL:       %s\n  • URL:            %s",
			daemonText,
			daemonInfo.IdleTTL,
			daemonInfo.URL,
		)

		fmt.Println(ui.Card(
			ui.BadgeSuccess("🤖 robo • System & Model Status"),
			sb.String(),
			"",
		))
		return nil
	}

	// Plain text output
	fmt.Printf("Config:    %s (exists: %v, mode: %s)\n", targetConfigPath, configExists, cfg.Robo.InferenceMode)
	fmt.Printf("SLM:       %s (backend: %s, model_found: %v, lib_found: %v)\n", slmStatus.Model, slmStatus.Backend, slmStatus.ModelFound, slmStatus.LibraryFound)
	if llmStatus.Configured || cfg.Robo.InferenceMode == "llm" || cfg.LLM.IsConfigured() {
		fmt.Printf("LLM:       %s (configured: %v)\n", llmStatus.Model, llmStatus.Configured)
	}
	fmt.Printf("Robod:     running: %v (pid: %d)\n", daemonInfo.Running, daemonInfo.PID)

	return nil
}
