package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zc/tchat/internal/provider"
	"gopkg.in/yaml.v3"
)

// Defaults holds default parameters for chat requests.
type Defaults struct {
	Temperature  float64 `yaml:"temperature"`
	MaxTokens    int     `yaml:"max_tokens"`
	Streaming    bool    `yaml:"streaming"`
	SystemPrompt string  `yaml:"system_prompt"`
}

// Config is the top-level configuration for tchat.
type Config struct {
	DefaultModel string                             `yaml:"default_model"`
	Defaults     Defaults                           `yaml:"defaults"`
	Providers    map[string]provider.ProviderConfig `yaml:"providers"`
	Aliases      map[string]string                  `yaml:"aliases"`
}

// ConfigPath returns the path to the config file (~/.config/tchat/config.yaml).
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "tchat", "config.yaml")
}

// Load reads and parses the config file from ~/.config/tchat/config.yaml.
// Returns an error if the file does not exist or cannot be parsed.
func Load() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

// applyDefaults fills in sensible defaults for any missing fields.
func applyDefaults(cfg *Config) {
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "openai/gpt-4o"
	}
	if cfg.Defaults.Temperature == 0 {
		cfg.Defaults.Temperature = 0.7
	}
	if cfg.Defaults.MaxTokens == 0 {
		cfg.Defaults.MaxTokens = 4096
	}
	// Streaming defaults to true. Since bool zero-value is false, users
	// must explicitly set "streaming: false" to disable it.
	if !cfg.Defaults.Streaming {
		cfg.Defaults.Streaming = true
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]provider.ProviderConfig)
	}
	if cfg.Aliases == nil {
		cfg.Aliases = make(map[string]string)
	}
}

// ResolveModel resolves a model name (which may be an alias or a provider/model
// string) into a provider name and model name.
//
// Examples:
//
//	"claude"          → ("anthropic", "claude-sonnet-4-20250514") via aliases
//	"openai/gpt-4o"  → ("openai", "gpt-4o")
//	"gpt-4o"         → ("openai", "gpt-4o") if found in a provider's model list
func (c *Config) ResolveModel(name string) (providerName, modelName string, err error) {
	// 1. Check aliases first.
	if target, ok := c.Aliases[name]; ok {
		name = target
	}

	// 2. Try provider/model format.
	if parts := strings.SplitN(name, "/", 2); len(parts) == 2 {
		prov := parts[0]
		model := parts[1]
		if _, ok := c.Providers[prov]; !ok {
			return "", "", fmt.Errorf("unknown provider %q", prov)
		}
		return prov, model, nil
	}

	// 3. Search all providers for a matching model name.
	for prov, pcfg := range c.Providers {
		for _, m := range pcfg.Models {
			if m == name {
				return prov, name, nil
			}
		}
	}

	return "", "", fmt.Errorf("cannot resolve model %q: not found in aliases or providers", name)
}
