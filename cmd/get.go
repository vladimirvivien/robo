package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/ui"
)

var (
	flagGetModel      string
	flagGetLibVersion string
	flagGetCacheDir   string
	flagGetNoUI       bool
	flagGetQuiet      bool
	flagGetForce      bool
)

// GetCmd represents the "robo get" asset provisioning subcommand.
var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Download local models and runtime libraries",
	Long: `Downloads on-device model weights from Hugging Face and native LiteRT-LM shared libraries.

Examples:
  robo get --model litert-community/gemma-4-E4B-it
  robo get --model gemma-4-e2b
  robo get --litertlm-lib v0.16.0
  robo get --model gemma-4-e2b --litertlm-lib v0.16.0 --no-ui`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runGet,
}

func init() {
	GetCmd.Flags().StringVar(&flagGetModel, "model", "", "Hugging Face model identifier, alias, or URL to download")
	GetCmd.Flags().StringVar(&flagGetLibVersion, "litertlm-lib", "", "LiteRT-LM runtime library version to download (e.g. v0.16.0)")
	GetCmd.Flags().StringVar(&flagGetLibVersion, "lib", "", "alias for --litertlm-lib")
	GetCmd.Flags().StringVar(&flagGetCacheDir, "cache-dir", "", "destination cache directory (default: ~/.robo/cache)")
	GetCmd.Flags().BoolVar(&flagGetNoUI, "no-ui", false, "hide progress bar and render plain text output")
	GetCmd.Flags().BoolVarP(&flagGetQuiet, "quiet", "q", false, "alias for --no-ui")
	GetCmd.Flags().BoolVarP(&flagGetForce, "force", "f", false, "force re-download and overwrite existing cached files")

	RootCmd.AddCommand(GetCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	modelInput := strings.TrimSpace(flagGetModel)
	libVersion := strings.TrimSpace(flagGetLibVersion)

	if modelInput == "" && libVersion == "" {
		return errors.New("please specify at least one asset to download (--model <name> or --litertlm-lib <version>)")
	}

	showUI := !flagGetNoUI && !flagGetQuiet && ui.IsStdoutTerminal()

	// Resolve cache directory
	targetCacheDir := flagGetCacheDir
	if targetCacheDir == "" {
		cfg, _ := config.Load(cfgFile)
		if cfg != nil && cfg.SLM.CacheDir != "" {
			targetCacheDir = cfg.SLM.CacheDir
		}
	}

	// 1. Download LiteRT-LM shared libraries if requested
	if libVersion != "" {
		if !flagGetForce && engine.IsLibDownloaded(libVersion) {
			if showUI {
				fmt.Println(ui.BadgeSuccess(fmt.Sprintf("✓ LiteRT-LM %s runtime libraries already cached", libVersion)))
			} else {
				fmt.Printf("LiteRT-LM %s runtime libraries already cached\n", libVersion)
			}
		} else {
			libDir, err := engine.DownloadLibAsset(ctx, libVersion, "", flagGetForce, showUI)
			if err != nil {
				return fmt.Errorf("download library %s: %w", libVersion, err)
			}
			if showUI {
				fmt.Println(ui.BadgeSuccess(fmt.Sprintf("✓ LiteRT-LM %s runtime libraries ready (%s)", libVersion, libDir)))
			} else {
				fmt.Printf("LiteRT-LM %s runtime libraries ready (%s)\n", libVersion, libDir)
			}
		}
	}

	// 2. Download model weights if requested
	if modelInput != "" {
		if !flagGetForce {
			foundPath := engine.FindLocalModelPath(modelInput, targetCacheDir)
			if foundPath != "" {
				if showUI {
					fmt.Println(ui.BadgeSuccess(fmt.Sprintf("✓ Model %s already cached at %s", modelInput, foundPath)))
				} else {
					fmt.Printf("Model %s already cached at %s\n", modelInput, foundPath)
				}
				return nil
			}
		}

		cachedPath, err := engine.DownloadModelAsset(ctx, modelInput, targetCacheDir, flagGetForce, showUI)
		if err != nil {
			return fmt.Errorf("download model %s: %w", modelInput, err)
		}
		if showUI {
			fmt.Println(ui.BadgeSuccess(fmt.Sprintf("✓ Model ready at %s", cachedPath)))
		} else {
			fmt.Printf("Model ready at %s\n", cachedPath)
		}
	}

	return nil
}
