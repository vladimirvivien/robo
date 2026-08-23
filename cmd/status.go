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
	Local        LocalStatusInfo  `json:"local"`
	Cloud        CloudStatusInfo  `json:"cloud"`
	Daemon       DaemonStatusInfo `json:"daemon"`
}

// LocalStatusInfo describes the on-device LiteRT-LM model and runtime status.
type LocalStatusInfo struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Backend      string `json:"backend"`
	Version      string `json:"version"`
	ModelPath    string `json:"model_path,omitempty"`
	ModelFound   bool   `json:"model_found"`
	LibraryFound bool   `json:"library_found"`
}

// CloudStatusInfo describes the frontier cloud model configuration and credentials.
type CloudStatusInfo struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
	Configured bool   `json:"configured"`
}

// DaemonStatusInfo describes the background robod process status.
type DaemonStatusInfo struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Model   string `json:"model,omitempty"`
}

// StatusCmd represents the "robo status" subcommand.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display current model configuration, runtime dependencies, and daemon status",
	Long: `Inspects local LiteRT-LM models, runtime libraries, cloud API key configuration,
active configuration paths, and the background daemon process state.`,
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

	// Local model inspection
	localModelPath := engine.FindLocalModelPath(cfg.SLM.Model, cfg.SLM.CacheDir)
	localLibFound := engine.IsLibDownloaded(cfg.SLM.Version)
	localStatus := LocalStatusInfo{
		Enabled:      true,
		Provider:     "litertlm",
		Model:        cfg.SLM.Model,
		Backend:      cfg.SLM.Backend,
		Version:      cfg.SLM.Version,
		ModelPath:    localModelPath,
		ModelFound:   localModelPath != "",
		LibraryFound: localLibFound,
	}

	// Cloud model inspection
	cloudCheck := engine.CheckCloudSetup(cfg.LLM)
	cloudStatus := CloudStatusInfo{
		Enabled:    cfg.Robo.InferenceMode == "llm" || cfg.LLM.APIKeyEnv != "",
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

	healthURL := config.DefaultRobodURL + "/health"
	if state != nil {
		healthURL = fmt.Sprintf("%s/health", state.URL)
		daemonPID = state.PID
		daemonModel = state.Model
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
		Running: daemonRunning,
		PID:     daemonPID,
		Model:   daemonModel,
	}

	report := StatusReport{
		ConfigPath:   targetConfigPath,
		ConfigExists: configExists,
		Local:        localStatus,
		Cloud:        cloudStatus,
		Daemon:       daemonInfo,
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
			fmt.Fprintf(&sb, "Config:\n  • File:       %s\n\n", targetConfigPath)
		} else {
			fmt.Fprintf(&sb, "Config:\n  • File:       %s (Not Found)\n\n", targetConfigPath)
			sb.WriteString("Status:\n  • robo is not initialized. Run 'robo init' to set up local models.\n")
			fmt.Println(ui.Card(
				ui.BadgeSuccess("🤖 robo • System & Model Status"),
				sb.String(),
				"",
			))
			return nil
		}

		// Local model section
		localModelStatus := "(Not Found - run 'robo init')"
		if localStatus.ModelFound {
			localModelStatus = fmt.Sprintf("%s (Ready)", localStatus.ModelPath)
		}

		localLibStatus := "(Missing - run 'robo init')"
		if localStatus.LibraryFound {
			localLibStatus = fmt.Sprintf("LiteRT-LM %s (Installed)", localStatus.Version)
		} else {
			localLibStatus = fmt.Sprintf("LiteRT-LM %s %s", localStatus.Version, localLibStatus)
		}

		fmt.Fprintf(&sb, "Local Model (LiteRT-LM):\n  • Model:      %s\n  • Backend:    %s\n  • Runtime:    %s\n  • Weights:    %s\n\n",
			localStatus.Model,
			localStatus.Backend,
			localLibStatus,
			localModelStatus,
		)

		// Cloud model section
		if cloudStatus.Enabled {
			cloudKeyStatus := fmt.Sprintf("%s (Not Set)", cloudStatus.APIKeyEnv)
			if cloudStatus.Configured {
				cloudKeyStatus = fmt.Sprintf("%s (Configured)", cloudStatus.APIKeyEnv)
			}
			fmt.Fprintf(&sb, "Cloud Model:\n  • Provider:   %s\n  • Model:      %s\n  • API Key:    %s\n\n",
				cloudStatus.Provider,
				cloudStatus.Model,
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
		fmt.Fprintf(&sb, "Daemon (robod):\n  • State:      %s", daemonText)

		fmt.Println(ui.Card(
			ui.BadgeSuccess("🤖 robo • System & Model Status"),
			sb.String(),
			"",
		))
		return nil
	}

	// Plain text output
	fmt.Printf("Config:    %s (exists: %v)\n", targetConfigPath, configExists)
	fmt.Printf("Local:     %s (backend: %s, model_found: %v, lib_found: %v)\n", localStatus.Model, localStatus.Backend, localStatus.ModelFound, localStatus.LibraryFound)
	if cloudStatus.Enabled {
		cloudModelStr := cloudStatus.Model
		if !strings.HasPrefix(cloudStatus.Model, cloudStatus.Provider+"/") {
			cloudModelStr = fmt.Sprintf("%s/%s", cloudStatus.Provider, cloudStatus.Model)
		}
		fmt.Printf("Cloud:     %s (configured: %v)\n", cloudModelStr, cloudStatus.Configured)
	}
	fmt.Printf("Daemon:    running: %v (pid: %d)\n", daemonInfo.Running, daemonInfo.PID)

	return nil
}
