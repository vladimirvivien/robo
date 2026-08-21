package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vladimirvivien/litertlm-go/pkg/libfetch"
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

// ResolveCacheDir returns the configured cache directory or the default ~/.robo/cache.
func ResolveCacheDir(customCacheDir string) string {
	if customCacheDir != "" {
		return customCacheDir
	}
	return config.DefaultCacheDir()
}

// ResolveLibDir returns the configured library directory or the default ~/.robo/lib/<version>.
func ResolveLibDir(version string, customLibDir ...string) string {
	if len(customLibDir) > 0 && customLibDir[0] != "" {
		return customLibDir[0]
	}
	return config.DefaultLibDir(version)
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

// CheckLocalSetup checks if local inference configuration is valid and provisioned on disk strictly in robo managed paths.
func CheckLocalSetup(cfg config.LocalConfig) LocalSetupStatus {
	resolvedLibDir := ResolveLibDir(cfg.Version, cfg.LibDir)
	resolvedModelPath := FindLocalModelPath(cfg.Model, cfg.CacheDir)

	hasLib := IsLibDownloaded(cfg.Version, cfg.LibDir)
	hasModel := resolvedModelPath != ""

	return LocalSetupStatus{
		Provisioned: hasLib && hasModel,
		HasLib:      hasLib,
		HasModel:    hasModel,
		LibDir:      resolvedLibDir,
		ModelPath:   resolvedModelPath,
	}
}

// IsLibDownloaded checks if LiteRT-LM shared libraries exist in the robo managed directory.
func IsLibDownloaded(version string, customLibDir ...string) bool {
	dir := ResolveLibDir(version, customLibDir...)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	return true
}

// FindLocalModelPath returns the absolute path to a cached model file, strictly searching robo cache paths.
func FindLocalModelPath(modelIDOrPath string, customCacheDir string) string {
	if strings.TrimSpace(modelIDOrPath) == "" {
		return ""
	}
	// 1. Explicit direct file path on disk
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

	cacheDir := ResolveCacheDir(customCacheDir)

	// Search strictly within the robo cache directory
	for _, name := range candidates {
		if name == "" {
			continue
		}
		// Direct file: <cacheDir>/<name>
		full := filepath.Join(cacheDir, name)
		if fileExists(full) && !isDirectory(full) {
			return full
		}
		// Legacy nested folder: <cacheDir>/<name>/<name>
		namespaced := filepath.Join(cacheDir, name, name)
		if fileExists(namespaced) && !isDirectory(namespaced) {
			return namespaced
		}
	}

	return ""
}

// IsModelDownloaded checks if the specified model is present in the robo cache directory.
func IsModelDownloaded(modelIDOrPath string) bool {
	return FindLocalModelPath(modelIDOrPath, "") != ""
}

// EnsureLocalSetup ensures the local libraries and model are downloaded.
func EnsureLocalSetup(ctx context.Context, cfg config.LocalConfig) (string, string, error) {
	return EnsureLocalSetupWithProgress(ctx, cfg)
}

// EnsureLocalSetupWithProgress downloads libraries and models into robo managed paths with visual terminal feedback.
func EnsureLocalSetupWithProgress(ctx context.Context, cfg config.LocalConfig) (string, string, error) {
	libDir := ResolveLibDir(cfg.Version, cfg.LibDir)
	modelPath := cfg.Model

	// 1. Provision native shared libraries into ~/.robo/lib/<version>
	if cfg.AutoDownload {
		libVersion := cfg.Version
		if libVersion == "" {
			libVersion = config.DefaultLocalVersion
		}

		if !IsLibDownloaded(libVersion, cfg.LibDir) {
			targetLibDir, err := DownloadLibAsset(ctx, libVersion, cfg.LibDir, false, ui.IsStdoutTerminal())
			if err != nil {
				return "", "", fmt.Errorf("local: fetch library: %w", err)
			}
			libDir = targetLibDir
		}
	}

	// 2. Provision model weights into ~/.robo/cache
	foundPath := FindLocalModelPath(modelPath, cfg.CacheDir)
	if foundPath != "" {
		modelPath = foundPath
	} else if cfg.AutoDownload {
		cachedPath, err := DownloadModelAsset(ctx, modelPath, cfg.CacheDir, false, ui.IsStdoutTerminal())
		if err != nil {
			return "", "", fmt.Errorf("local: fetch model %q: %w", modelPath, err)
		}
		modelPath = cachedPath
	}

	return libDir, modelPath, nil
}

// DownloadModelAsset downloads a model by identifier/alias/URL directly to the robo cache directory.
func DownloadModelAsset(ctx context.Context, modelIDOrURL string, customCacheDir string, force bool, showUI bool) (string, error) {
	if strings.TrimSpace(modelIDOrURL) == "" {
		return "", fmt.Errorf("model identifier cannot be empty")
	}

	targetURL, targetFilename := ResolveModelTarget(modelIDOrURL)
	if targetURL == "" {
		return "", fmt.Errorf("unable to resolve model %q", modelIDOrURL)
	}

	cacheDir := ResolveCacheDir(customCacheDir)

	if !force {
		found := FindLocalModelPath(modelIDOrURL, cacheDir)
		if found != "" {
			return found, nil
		}
	}

	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return "", fmt.Errorf("create model cache dir: %w", err)
	}

	destPath := filepath.Join(cacheDir, targetFilename)
	skipIfExists := fileExists(destPath) && !force

	var pb *ui.ProgressBar
	var opts []modelfetch.Option
	opts = append(opts,
		modelfetch.WithDir(cacheDir),
		modelfetch.WithFilename(targetFilename),
		modelfetch.WithSkipIfExists(skipIfExists),
	)

	if showUI && ui.IsStdoutTerminal() {
		pb = ui.NewProgressBar(fmt.Sprintf("Downloading %s", targetFilename))
		opts = append(opts, modelfetch.WithProgress(func(downloaded, total int64, pct float64) {
			pb.Update(downloaded, total, pct)
		}))
	}

	cachedPath, err := modelfetch.Fetch(ctx, targetURL, opts...)
	if pb != nil {
		pb.Finish(fmt.Sprintf("✓ Downloaded %s", targetFilename))
	}
	if err != nil {
		return "", fmt.Errorf("fetch model %q: %w", modelIDOrURL, err)
	}

	return cachedPath, nil
}

// DownloadLibAsset downloads the LiteRT-LM shared libraries directly to the robo lib directory.
func DownloadLibAsset(ctx context.Context, version string, customLibDir string, force bool, showUI bool) (string, error) {
	if version == "" {
		version = config.DefaultLocalVersion
	}

	targetDir := ResolveLibDir(version, customLibDir)

	if !force && IsLibDownloaded(version, customLibDir) {
		return targetDir, nil
	}

	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return "", fmt.Errorf("create lib dir: %w", err)
	}

	var sp *ui.Spinner
	if showUI && ui.IsStdoutTerminal() {
		sp = ui.StartSpinner(fmt.Sprintf("Downloading LiteRT-LM native runtime libraries (%s)...", version))
	}

	opts := []libfetch.Option{
		libfetch.WithDir(targetDir),
		libfetch.WithVersion(version),
	}

	dir, err := libfetch.Fetch(ctx, opts...)
	if sp != nil {
		sp.Stop()
	}
	if err != nil {
		return "", fmt.Errorf("fetch library: %w", err)
	}

	return dir, nil
}

// ValidateInferenceSetup verifies backend availability before running requests and fails fast if dependencies are missing.
func ValidateInferenceSetup(cfg *config.Config, forceBackend string) error {
	cloudStatus := CheckCloudSetup(cfg.LLM.Cloud)
	localStatus := CheckLocalSetup(cfg.LLM.Local)

	target := strings.ToLower(forceBackend)

	// 1. LiteRT-LM runtime library is always required
	if !localStatus.HasLib {
		return fmt.Errorf("LiteRT-LM runtime library (%s) was not found on disk at %s.\n\nTo resolve:\n  • Run 'robo init' or 'robo get --litertlm-lib %s' to download libraries\n  • Or specify 'llm.local.lib_dir' in %s",
			cfg.LLM.Local.Version, localStatus.LibDir, cfg.LLM.Local.Version, config.ConfigPath())
	}

	// 2. Local model weights are always required (local litertlm is mandatory)
	if !localStatus.HasModel {
		return fmt.Errorf("configured local model %q was not found on disk.\n\nTo resolve:\n  • Run 'robo init' or 'robo get --model %s' to download model weights\n  • Or update 'llm.local.model' in %s with a valid local path",
			cfg.LLM.Local.Model, cfg.LLM.Local.Model, config.ConfigPath())
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
