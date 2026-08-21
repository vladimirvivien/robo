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

// CheckLocalSetup checks if local inference configuration is valid and provisioned on disk.
func CheckLocalSetup(cfg config.LocalConfig) LocalSetupStatus {
	status := LocalSetupStatus{
		LibDir:    cfg.LibDir,
		ModelPath: cfg.Model,
	}

	if status.LibDir != "" && fileExists(status.LibDir) {
		status.HasLib = true
	} else if IsLibDownloaded(cfg.Version) {
		status.HasLib = true
	}

	if FindLocalModelPath(status.ModelPath, cfg.CacheDir) != "" {
		status.HasModel = true
	}

	status.Provisioned = status.HasLib && status.HasModel
	return status
}

// IsLibDownloaded checks if LiteRT-LM shared libraries are already cached locally on disk.
func IsLibDownloaded(version string) bool {
	if version == "" {
		version = config.DefaultLocalVersion
	}
	dir, err := libfetch.DefaultDir(version)
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	return true
}

// FindLocalModelPath returns the absolute path to a cached or local model file on disk.
func FindLocalModelPath(modelIDOrPath string, customCacheDir string) string {
	if modelIDOrPath == "" {
		return ""
	}
	if fileExists(modelIDOrPath) && !isDirectory(modelIDOrPath) {
		return modelIDOrPath
	}

	var candidates []string
	if info, ok := LookupModel(modelIDOrPath); ok {
		candidates = append(candidates, info.Filename)
	}
	candidates = append(candidates, filepath.Base(modelIDOrPath))
	if !strings.HasSuffix(strings.ToLower(modelIDOrPath), ".litertlm") {
		candidates = append(candidates, filepath.Base(modelIDOrPath)+".litertlm")
	}

	searchDirs := []string{}
	if customCacheDir != "" {
		searchDirs = append(searchDirs, customCacheDir)
	}
	if home, err := os.UserHomeDir(); err == nil {
		searchDirs = append(searchDirs,
			filepath.Join(home, ".robo", "cache"),
		)
	}
	if cacheDir, err := modelfetch.DefaultCacheDir(); err == nil {
		searchDirs = append(searchDirs, cacheDir)
	}

	// Search prioritized candidate filenames across directories
	for _, name := range candidates {
		if name == "" {
			continue
		}
		for _, dir := range searchDirs {
			// 1. Check namespaced folder: <dir>/<name>/<name>
			namespaced := filepath.Join(dir, name, name)
			if fileExists(namespaced) && !isDirectory(namespaced) {
				return namespaced
			}
			// 2. Check direct file: <dir>/<name>
			full := filepath.Join(dir, name)
			if fileExists(full) && !isDirectory(full) {
				return full
			}
		}
	}

	return ""
}

// IsModelDownloaded checks if the specified model is present on disk or in local cache locations.
func IsModelDownloaded(modelIDOrPath string) bool {
	return FindLocalModelPath(modelIDOrPath, "") != ""
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
			if ui.IsStdoutTerminal() {
				sp := ui.StartSpinner("Downloading LiteRT-LM native runtime libraries...")
				dir, err := litertlm.FetchLib(runtime.GOOS, runtime.GOARCH, libVersion)
				sp.Stop()
				if err != nil {
					return "", "", fmt.Errorf("local: fetch library: %w", err)
				}
				libDir = dir
				fmt.Println(ui.BadgeSuccess("✓ LiteRT-LM runtime libraries ready"))
			} else {
				dir, err := litertlm.FetchLib(runtime.GOOS, runtime.GOARCH, libVersion)
				if err != nil {
					return "", "", fmt.Errorf("local: fetch library: %w", err)
				}
				libDir = dir
			}
		} else {
			dir, err := libfetch.DefaultDir(libVersion)
			if err == nil {
				libDir = dir
			}
		}
	}

	// 2. Provision model weights if needed
	foundPath := FindLocalModelPath(modelPath, cfg.CacheDir)
	if foundPath != "" {
		modelPath = foundPath
	} else if cfg.AutoDownload {
		targetURL, targetFilename := ResolveModelTarget(modelPath)
		if targetURL == "" {
			return "", "", fmt.Errorf("local: unable to resolve model %q", modelPath)
		}

		cacheDir := cfg.CacheDir
		if cacheDir == "" {
			if home, err := os.UserHomeDir(); err == nil {
				cacheDir = filepath.Join(home, ".robo", "cache")
			} else {
				cacheDir, _ = modelfetch.DefaultCacheDir()
			}
		}

		// Namespaced model folder: ~/.robo/cache/gemma-4-2B-it.litertlm/
		modelFolder := filepath.Join(cacheDir, targetFilename)
		destPath := filepath.Join(modelFolder, targetFilename)
		directPath := filepath.Join(cacheDir, targetFilename)

		if fileExists(destPath) && !isDirectory(destPath) {
			modelPath = destPath
		} else if fileExists(directPath) && !isDirectory(directPath) {
			modelPath = directPath
		} else {
			if err := os.MkdirAll(modelFolder, 0750); err != nil {
				return "", "", fmt.Errorf("local: create model cache dir: %w", err)
			}
			var pb *ui.ProgressBar
			var opts []modelfetch.Option
			opts = append(opts,
				modelfetch.WithDir(modelFolder),
				modelfetch.WithFilename(targetFilename),
				modelfetch.WithSkipIfExists(false),
			)

			if ui.IsStdoutTerminal() {
				pb = ui.NewProgressBar(fmt.Sprintf("Downloading %s", targetFilename))
				opts = append(opts, modelfetch.WithProgress(func(downloaded, total int64, pct float64) {
					pb.Update(downloaded, total, pct)
				}))
			}

			cachedPath, err := litertlm.FetchModel(ctx, targetURL, opts...)
			if pb != nil {
				pb.Finish(fmt.Sprintf("✓ Downloaded %s", targetFilename))
			}
			if err != nil {
				return "", "", fmt.Errorf("local: fetch model %q: %w", modelPath, err)
			}
			modelPath = cachedPath
		}
	}

	return libDir, modelPath, nil
}

// ValidateInferenceSetup verifies backend availability before running requests and fails fast if dependencies are missing.
func ValidateInferenceSetup(cfg *config.Config, forceBackend string) error {
	cloudStatus := CheckCloudSetup(cfg.LLM.Cloud)
	localStatus := CheckLocalSetup(cfg.LLM.Local)

	target := strings.ToLower(forceBackend)

	// 1. LiteRT-LM runtime library is always required
	if !localStatus.HasLib {
		return fmt.Errorf("LiteRT-LM runtime library (%s) was not found on disk.\n\nTo resolve:\n  • Run 'robo init' to download the runtime library\n  • Or specify 'llm.local.lib_dir' in %s", cfg.LLM.Local.Version, config.ConfigPath())
	}

	// 2. Local model weights are always required (local litertlm is mandatory)
	if !localStatus.HasModel {
		return fmt.Errorf("configured local model %q was not found on disk.\n\nTo resolve:\n  • Run 'robo init' to download the model weights\n  • Or update 'llm.local.model' in %s with a valid local path", cfg.LLM.Local.Model, config.ConfigPath())
	}

	// 3. If cloud-only was explicitly requested, ensure cloud credentials are configured
	if target == "cloud-only" || target == "cloud" {
		if !cloudStatus.Configured {
			return fmt.Errorf("cloud inference is not configured: missing API key for provider %q.\n\nTo resolve:\n  • Set %s in your environment", cfg.LLM.Cloud.Provider, cloudStatus.APIKeyEnv)
		}
	}

	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func isDirectory(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}
