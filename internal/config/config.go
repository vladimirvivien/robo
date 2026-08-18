package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default constants for configuration
const (
	DefaultConfigDir       = ".config/robo"
	DefaultConfigFile      = "config.yaml"
	DefaultLocalModel      = "litert-community/gemma3-1b-it-int4"
	DefaultCloudModel      = "googleai/gemini-2.5-flash"
	DefaultCloudProvider   = "googleai"
	DefaultLocalBackend    = "gpu"
	DefaultRoutingStrategy = "auto"
	DefaultMaxLocalTokens  = 4096
	DefaultDaemonPort      = 8765
	DefaultDaemonIdleTTL   = 15 * time.Minute
	DefaultOutputMode      = "markdown"
	DefaultSessionMode     = "daily"
	DefaultShellAlias      = "ai"
)

// Config represents the complete Robo configuration.
type Config struct {
	OutputMode     string        `yaml:"output_mode"`
	DefaultSession string        `yaml:"default_session"`
	Routing        RoutingConfig `yaml:"routing"`
	Local          LocalConfig   `yaml:"local"`
	Daemon         DaemonConfig  `yaml:"daemon"`
	Cloud          CloudConfig   `yaml:"cloud"`
	Shell          ShellConfig   `yaml:"shell"`
}

// RoutingConfig controls hybrid model routing decisions.
type RoutingConfig struct {
	Strategy        string `yaml:"strategy"` // "auto", "local-first", "cloud-first", "local-only", "cloud-only"
	EscalateOnError bool   `yaml:"escalate_on_error"`
	MaxLocalTokens  int    `yaml:"max_local_tokens"`
}

// LocalConfig defines settings for the on-device LiteRT-LM engine.
type LocalConfig struct {
	Provider     string `yaml:"provider"` // "litertlm"
	Model        string `yaml:"model"`
	Backend      string `yaml:"backend"` // "gpu", "cpu"
	AutoDownload bool   `yaml:"auto_download"`
	CacheDir     string `yaml:"cache_dir"`
	LibDir       string `yaml:"lib_dir"`
}

// DaemonConfig defines settings for the hot-start background daemon.
type DaemonConfig struct {
	Enabled bool          `yaml:"enabled"`
	IdleTTL time.Duration `yaml:"idle_ttl"`
	Port    int           `yaml:"port"`
}

// CloudConfig defines settings for the Genkit cloud engine.
type CloudConfig struct {
	Provider  string `yaml:"provider"` // "googleai", "anthropic", "openai"
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	APIKeyEnv string `yaml:"api_key_env"`
}

// ShellConfig defines integration settings for Bash/Zsh/Fish/PowerShell.
type ShellConfig struct {
	Alias              string `yaml:"alias"`
	CaptureLastCommand bool   `yaml:"capture_last_command"`
	CaptureExitCode    bool   `yaml:"capture_exit_code"`
}

// NewDefaultConfig returns a Config struct initialized with standard defaults.
func NewDefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "robo")

	return &Config{
		OutputMode:     DefaultOutputMode,
		DefaultSession: DefaultSessionMode,
		Routing: RoutingConfig{
			Strategy:        DefaultRoutingStrategy,
			EscalateOnError: true,
			MaxLocalTokens:  DefaultMaxLocalTokens,
		},
		Local: LocalConfig{
			Provider:     "litertlm",
			Model:        DefaultLocalModel,
			Backend:      DefaultLocalBackend,
			AutoDownload: true,
			CacheDir:     filepath.Join(configDir, "cache"),
		},
		Daemon: DaemonConfig{
			Enabled: true,
			IdleTTL: DefaultDaemonIdleTTL,
			Port:    DefaultDaemonPort,
		},
		Cloud: CloudConfig{
			Provider:  DefaultCloudProvider,
			Model:     DefaultCloudModel,
			APIKeyEnv: "GEMINI_API_KEY",
		},
		Shell: ShellConfig{
			Alias:              DefaultShellAlias,
			CaptureLastCommand: true,
			CaptureExitCode:    true,
		},
	}
}

// ConfigPath returns the standard path to config.yaml in the user's config dir.
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfigFile
	}
	return filepath.Join(home, ".config", "robo", DefaultConfigFile)
}

// Load reads and parses the configuration file at the given path, or the default
// location if path is empty. If the file does not exist, default settings are returned.
func Load(path string) (*Config, error) {
	if path == "" {
		path = ConfigPath()
	}

	cfg := NewDefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.applyEnvOverrides()
			return cfg, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	cfg.applyEnvOverrides()
	return cfg, nil
}

// Save writes the configuration to disk at the specified path.
func (c *Config) Save(path string) error {
	if path == "" {
		path = ConfigPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config: create dir %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}

	return nil
}

// applyEnvOverrides merges environment variables into configuration fields.
func (c *Config) applyEnvOverrides() {
	if env := os.Getenv("ROBO_ROUTING_STRATEGY"); env != "" {
		c.Routing.Strategy = env
	}
	if env := os.Getenv("ROBO_LOCAL_MODEL"); env != "" {
		c.Local.Model = env
	}
	if env := os.Getenv("ROBO_LOCAL_BACKEND"); env != "" {
		c.Local.Backend = env
	}
	if env := os.Getenv("ROBO_CLOUD_MODEL"); env != "" {
		c.Cloud.Model = env
	}
	if env := os.Getenv("ROBO_CLOUD_PROVIDER"); env != "" {
		c.Cloud.Provider = env
	}
	if env := os.Getenv("LITERTLM_LIB"); env != "" {
		c.Local.LibDir = env
	}

	// Resolve API key from environment variable name if configured
	if c.Cloud.APIKeyEnv != "" && c.Cloud.APIKey == "" {
		c.Cloud.APIKey = os.Getenv(c.Cloud.APIKeyEnv)
	}
	if c.Cloud.APIKey == "" {
		// Fallback check standard provider envs
		switch strings.ToLower(c.Cloud.Provider) {
		case "googleai", "gemini":
			if key := os.Getenv("GEMINI_API_KEY"); key != "" {
				c.Cloud.APIKey = key
			}
		case "anthropic", "claude":
			if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
				c.Cloud.APIKey = key
			}
		case "openai":
			if key := os.Getenv("OPENAI_API_KEY"); key != "" {
				c.Cloud.APIKey = key
			}
		}
	}
}
