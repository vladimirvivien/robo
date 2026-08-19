package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vladimirvivien/litertlm-go/pkg/libfetch"
	"github.com/vladimirvivien/litertlm-go/pkg/litertlm"
	"github.com/vladimirvivien/litertlm-go/pkg/modelfetch"
	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/ui"
)

// CloudSetupStatus reports whether cloud inference credentials are set up.
type CloudSetupStatus struct {
	Configured bool
	Provider   string
	Model      string
	APIKeyEnv  string
}

// LocalSetupStatus reports whether local on-device inference files are set up.
type LocalSetupStatus struct {
	Provisioned bool
	HasLib      bool
	HasModel    bool
	LibDir      string
	ModelPath   string
}

// CheckCloudSetup checks if cloud model credentials (API key) are available.
func CheckCloudSetup(cfg config.CloudConfig) CloudSetupStatus {
	apiKey := cfg.APIKey
	apiKeyEnv := cfg.APIKeyEnv

	if apiKeyEnv == "" {
		switch strings.ToLower(cfg.Provider) {
		case "anthropic", "claude":
			apiKeyEnv = "ANTHROPIC_API_KEY"
		case "openai":
			apiKeyEnv = "OPENAI_API_KEY"
		default:
			apiKeyEnv = "GEMINI_API_KEY"
		}
	}

	if apiKey == "" && apiKeyEnv != "" {
		apiKey = os.Getenv(apiKeyEnv)
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return CloudSetupStatus{
		Configured: apiKey != "",
		Provider:   cfg.Provider,
		Model:      cfg.Model,
		APIKeyEnv:  apiKeyEnv,
	}
}

// CheckLocalSetup checks if local inference configuration is valid and provisioned.
func CheckLocalSetup(cfg config.LocalConfig) LocalSetupStatus {
	status := LocalSetupStatus{
		LibDir:    cfg.LibDir,
		ModelPath: cfg.Model,
	}

	if status.LibDir != "" && fileExists(status.LibDir) {
		status.HasLib = true
	} else if cfg.AutoDownload {
		status.HasLib = true
	}

	if filepath.IsAbs(status.ModelPath) && fileExists(status.ModelPath) {
		status.HasModel = true
	} else if cfg.AutoDownload {
		status.HasModel = true
	}

	status.Provisioned = status.HasLib && status.HasModel
	return status
}

// IsLibDownloaded checks if LiteRT-LM shared libraries are already cached locally.
func IsLibDownloaded(version string) bool {
	if version == "" {
		version = config.DefaultLocalVersion
	}
	dir, err := libfetch.DefaultDir(version)
	if err != nil {
		return false
	}
	platform, err := libfetch.Platform()
	if err != nil {
		return false
	}
	targetDir := filepath.Join(dir, platform)
	entries, err := os.ReadDir(targetDir)
	if err != nil || len(entries) == 0 {
		return false
	}
	return true
}

// IsModelDownloaded checks if the specified model is present on disk or in the local cache.
func IsModelDownloaded(modelIDOrPath string) bool {
	if modelIDOrPath == "" {
		return false
	}
	if fileExists(modelIDOrPath) {
		return true
	}
	target, err := modelfetch.ResolveModelIdentifier(modelIDOrPath)
	if err != nil {
		return false
	}
	cacheDir, err := modelfetch.DefaultCacheDir()
	if err != nil {
		return false
	}
	destPath := filepath.Join(cacheDir, target.Filename)
	return fileExists(destPath)
}

// EnsureLocalSetup ensures the local libraries and model are downloaded via litertlm-go.
func EnsureLocalSetup(ctx context.Context, cfg config.LocalConfig) (string, string, error) {
	return EnsureLocalSetupWithProgress(ctx, cfg)
}

// EnsureLocalSetupWithProgress downloads libraries and models with visual terminal feedback.
func EnsureLocalSetupWithProgress(ctx context.Context, cfg config.LocalConfig) (string, string, error) {
	libDir := cfg.LibDir
	modelPath := cfg.Model

	// 1. Provision native shared libraries if needed
	if libDir == "" && cfg.AutoDownload {
		libVersion := cfg.Version
		if libVersion == "" {
			libVersion = config.DefaultLocalVersion
		}

		if !IsLibDownloaded(libVersion) {
			sp := ui.StartSpinner("Downloading LiteRT-LM native runtime libraries...")
			dir, err := litertlm.FetchLib(runtime.GOOS, runtime.GOARCH, libVersion)
			sp.Stop()
			if err != nil {
				return "", "", fmt.Errorf("local: fetch library: %w", err)
			}
			libDir = dir
			fmt.Println(ui.BadgeSuccess("✓ LiteRT-LM runtime libraries ready"))
		} else {
			dir, err := libfetch.DefaultDir(libVersion)
			if err == nil {
				platform, _ := libfetch.Platform()
				libDir = filepath.Join(dir, platform)
			}
		}
	}

	// 2. Provision model weights if needed
	if !filepath.IsAbs(modelPath) && !fileExists(modelPath) && cfg.AutoDownload {
		target, err := modelfetch.ResolveModelIdentifier(modelPath)
		if err != nil {
			return "", "", fmt.Errorf("local: resolve model %q: %w", modelPath, err)
		}

		cacheDir := cfg.CacheDir
		if cacheDir == "" {
			cacheDir, _ = modelfetch.DefaultCacheDir()
		}
		destPath := filepath.Join(cacheDir, target.Filename)

		if !fileExists(destPath) {
			pb := ui.NewProgressBar(fmt.Sprintf("Downloading %s", target.Filename))
			cachedPath, err := litertlm.FetchModel(
				ctx,
				modelPath,
				litertlm.WithModelDir(cacheDir),
				litertlm.WithModelProgress(func(downloaded, total int64, pct float64) {
					pb.Update(downloaded, total, pct)
				}),
			)
			pb.Finish(fmt.Sprintf("✓ Downloaded %s", target.Filename))
			if err != nil {
				return "", "", fmt.Errorf("local: fetch model %q: %w", modelPath, err)
			}
			modelPath = cachedPath
		} else {
			modelPath = destPath
		}
	}

	return libDir, modelPath, nil
}

// RunInitialSetup informs the user that Robo is not configured, walks them through
// selecting an on-device model, downloads dependencies with visual progress,
// and saves the configuration.
func RunInitialSetup(ctx context.Context, cfg *config.Config) error {
	if !ui.IsStdoutTerminal() || !ui.IsStdinTerminal() {
		// Non-interactive: save default config and proceed
		return cfg.Save("")
	}

	// 1. Inform user
	fmt.Println()
	fmt.Println(ui.Card(
		ui.BadgeWarning("robo • Setup Required"),
		"Robo is not configured yet.\nLet's set up your local on-device language model to get started.",
		"Google LiteRT-LM • Private & On-Device",
	))
	fmt.Println()

	// 2. Prompt user to select model (Gemma 4 2B or Gemma 4 4B)
	selectedModel, err := ui.PromptModelSelection()
	if err != nil {
		return fmt.Errorf("setup cancelled: %w", err)
	}

	cfg.LLM.Local.Model = selectedModel

	// 3. Save configuration
	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.BadgeSuccess("✓ Configuration saved to " + config.ConfigPath()))
	fmt.Println()

	// 4. Download library and model dependencies with visual progress
	_, _, err = EnsureLocalSetupWithProgress(ctx, cfg.LLM.Local)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(ui.Card(
		ui.BadgeSuccess("Setup Complete"),
		"On-device LiteRT-LM model is downloaded and ready.\nContinuing with your request...",
		"",
	))
	fmt.Println()

	return nil
}

// ValidateInferenceSetup verifies backend availability before running requests and returns user-friendly guidance.
func ValidateInferenceSetup(cfg *config.Config, forceBackend string) error {
	cloudStatus := CheckCloudSetup(cfg.LLM.Cloud)
	localStatus := CheckLocalSetup(cfg.LLM.Local)

	target := strings.ToLower(forceBackend)

	switch target {
	case "local-only", "local":
		if !localStatus.Provisioned && !cfg.LLM.Local.AutoDownload {
			return fmt.Errorf("local inference is not set up (missing model/library in %s) and auto_download is disabled", cfg.LLM.Local.CacheDir)
		}
		return nil

	case "cloud-only", "cloud":
		if !cloudStatus.Configured {
			return fmt.Errorf("cloud inference is not set up: missing API key for provider %q (set %s in environment)", cfg.LLM.Cloud.Provider, cloudStatus.APIKeyEnv)
		}
		return nil

	default: // auto / hybrid
		// If local is enabled and can auto-download or is already provisioned
		if cfg.LLM.Local.Enabled {
			if localStatus.Provisioned || cfg.LLM.Local.AutoDownload {
				return nil
			}
		}
		// If cloud is enabled and configured
		if cfg.LLM.Cloud.Enabled && cloudStatus.Configured {
			return nil
		}
		if localStatus.Provisioned || cfg.LLM.Local.AutoDownload {
			return nil
		}
		if cloudStatus.Configured {
			return nil
		}
		return fmt.Errorf("no language model is set up: enable auto_download in config (default) or set %s", cloudStatus.APIKeyEnv)
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
