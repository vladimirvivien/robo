package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	flagInitVersion        string
	flagInitModel          string
	flagInitBackend        string
	flagInitNonInteractive bool
	flagInitForce          bool
)

// InitCmd represents the "robo init" setup subcommand.
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize robo, configure models, and download runtime dependencies",
	Long: `Performs pre-flight checks, provisions Google LiteRT-LM runtime dependencies (v0.16.0),
configures on-device Gemma 4 models, and generates configuration (default: ~/.robo/config.yaml).

To save configuration to a custom location, specify the --config flag:
  robo init --config /path/to/custom-config.yaml`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runInit,
}

func init() {
	InitCmd.Flags().StringVar(&flagInitVersion, "version", "", "LiteRT-LM runtime library version (default is v0.16.0)")
	InitCmd.Flags().StringVar(&flagInitModel, "model", "", "Gemma 4 model identifier to download (e.g. litert-community/gemma-4-E4B-it-litert-lm)")
	InitCmd.Flags().StringVar(&flagInitBackend, "backend", "", "hardware acceleration backend (gpu, cpu)")
	InitCmd.Flags().BoolVarP(&flagInitNonInteractive, "non-interactive", "y", false, "run non-interactively using default settings (Gemma 4 4B + GPU)")
	InitCmd.Flags().BoolVar(&flagInitForce, "force", false, "force re-initialization even if config file already exists")

	RootCmd.AddCommand(InitCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	targetConfigPath := cfgFile
	if targetConfigPath == "" {
		targetConfigPath = config.ConfigPath()
	}

	// 1. Check if already initialized
	if config.ConfigFileExists(targetConfigPath) && !flagInitForce {
		if !flagInitNonInteractive && ui.IsStdoutTerminal() && ui.IsStdinTerminal() {
			confirmed, err := ui.PromptConfirm(fmt.Sprintf("Configuration already exists at %s.\nRe-initialize?", targetConfigPath))
			if err != nil || !confirmed {
				fmt.Println("Initialization cancelled. Existing configuration preserved.")
				return nil
			}
		} else {
			fmt.Printf("robo is already initialized (%s). Use --force to re-initialize.\n", targetConfigPath)
			return nil
		}
	}

	cfg := config.NewDefaultConfig()

	// 2. Interactive selection or flag resolution
	selectedVersion := config.DefaultLocalVersion
	selectedModel := config.DefaultLocalModel
	selectedBackend := config.DefaultLocalBackend

	if flagInitVersion != "" {
		selectedVersion = flagInitVersion
	}
	if flagInitModel != "" {
		selectedModel = flagInitModel
	}
	if flagInitBackend != "" {
		selectedBackend = flagInitBackend
	}

	isInteractive := ui.IsStdoutTerminal() && ui.IsStdinTerminal() && !flagInitNonInteractive && flagInitVersion == "" && flagInitModel == "" && flagInitBackend == ""
	if isInteractive {
		fmt.Println()
		fmt.Println(ui.Card(
			ui.BadgeSuccess("🤖 robo • Setup & Initialization"),
			"Configure your local on-device language model.\nGoogle LiteRT-LM • Private & Offline",
			"",
		))
		fmt.Println()

		var choices []ui.ModelChoice
		for _, m := range engine.ModelCatalog {
			choices = append(choices, ui.ModelChoice{
				ID:          m.ID,
				Description: m.Description,
				Default:     m.Default,
			})
		}

		prefs, err := ui.PromptInitSelection(choices...)
		if err != nil {
			return fmt.Errorf("initialization cancelled: %w", err)
		}
		selectedVersion = prefs.Version
		selectedModel = prefs.Model
		selectedBackend = prefs.Backend
	}

	cfg.SLM.Version = selectedVersion
	cfg.SLM.Model = selectedModel
	cfg.SLM.Backend = selectedBackend
	cfg.SLM.AutoDownload = true

	// 3. Save default configuration file to disk
	if err := cfg.Save(targetConfigPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if isInteractive {
		fmt.Println()
		fmt.Println(ui.BadgeSuccess("✓ Configuration saved to " + targetConfigPath))
		fmt.Println()
	} else {
		fmt.Printf("Configuration saved to %s\n", targetConfigPath)
	}

	// 4. Download LiteRT-LM shared libraries and selected Gemma model
	_, _, err := engine.EnsureLocalSetupWithProgress(ctx, cfg.SLM)
	if err != nil {
		return fmt.Errorf("setup dependencies: %w", err)
	}

	// 5. Completion notice
	if isInteractive {
		fmt.Println()
		exampleCmd := `robo "which process is consuming the most cpu"`
		if cfgFile != "" {
			exampleCmd = fmt.Sprintf(`robo --config %s "which process is consuming the most cpu"`, targetConfigPath)
		}

		summaryLines := []string{
			fmt.Sprintf("• Config:         %s", targetConfigPath),
			fmt.Sprintf("• Local Model:    %s (LiteRT-LM %s, %s)", selectedModel, cfg.SLM.Version, cfg.SLM.Backend),
			fmt.Sprintf("• Inference Mode: %s", cfg.Robo.InferenceMode),
			fmt.Sprintf("\nTry running:\n  %s", exampleCmd),
		}

		summary := strings.Join(summaryLines, "\n")
		fmt.Println(ui.Card(
			ui.BadgeSuccess("🤖 robo • Initialization Complete"),
			summary,
			"",
		))
	} else {
		fmt.Printf("Initialization complete. On-device models ready (%s).\n", targetConfigPath)
	}

	return nil
}
