package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vladimirvivien/litertlm-go/pkg/litertlm"
	"github.com/vladimirvivien/robo/internal/config"
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

// EnsureLocalSetup ensures the local libraries and model are downloaded via litertlm-go.
func EnsureLocalSetup(ctx context.Context, cfg config.LocalConfig) (string, string, error) {
	libDir := cfg.LibDir
	modelPath := cfg.Model

	// 1. Provision native shared libraries if needed
	if libDir == "" && cfg.AutoDownload {
		libVersion := cfg.LibVersion
		if libVersion == "" {
			libVersion = config.DefaultLocalLibVersion
		}
		dir, err := litertlm.FetchLib(runtime.GOOS, runtime.GOARCH, libVersion)
		if err != nil {
			return "", "", fmt.Errorf("local: fetch library: %w", err)
		}
		libDir = dir
	}

	// 2. Provision model weights if needed
	if !filepath.IsAbs(modelPath) && !fileExists(modelPath) && cfg.AutoDownload {
		cachedPath, err := litertlm.FetchModel(ctx, modelPath)
		if err != nil {
			return "", "", fmt.Errorf("local: fetch model %q: %w", modelPath, err)
		}
		modelPath = cachedPath
	}

	return libDir, modelPath, nil
}

// ValidateInferenceSetup verifies backend availability before running requests and returns user-friendly guidance.
func ValidateInferenceSetup(cfg *config.Config, forceBackend string) error {
	cloudStatus := CheckCloudSetup(cfg.Cloud)
	localStatus := CheckLocalSetup(cfg.Local)

	target := strings.ToLower(forceBackend)
	if target == "" {
		target = strings.ToLower(cfg.Routing.Strategy)
	}

	switch target {
	case "local-only", "local":
		if !localStatus.Provisioned && !cfg.Local.AutoDownload {
			return fmt.Errorf("local inference is not set up (missing model/library in %s) and auto_download is disabled", cfg.Local.CacheDir)
		}
		return nil

	case "cloud-only", "cloud":
		if !cloudStatus.Configured {
			return fmt.Errorf("cloud inference is not set up: missing API key for provider %q (set %s in environment)", cfg.Cloud.Provider, cloudStatus.APIKeyEnv)
		}
		return nil

	default: // auto / hybrid
		// If local can auto-download or is already provisioned, auto mode is ready
		if localStatus.Provisioned || cfg.Local.AutoDownload {
			return nil
		}
		// If local cannot run, verify cloud
		if cloudStatus.Configured {
			return nil
		}
		// Neither is set up
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
