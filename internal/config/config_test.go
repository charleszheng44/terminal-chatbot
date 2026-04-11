package config

import (
	"strings"
	"testing"

	"github.com/zc/tchat/internal/provider"
	"gopkg.in/yaml.v3"
)

// loadFromBytes mirrors Load() but without touching the filesystem. Load()
// hardcodes ~/.config/tchat/config.yaml, so this helper exercises the
// unmarshal + applyDefaults path directly.
func loadFromBytes(t *testing.T, data []byte) (*Config, error) {
	t.Helper()
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

func TestLoadFromBytes_ValidYAML(t *testing.T) {
	data := []byte(`
default_model: anthropic/claude-sonnet-4-20250514
defaults:
  temperature: 0.3
  max_tokens: 2048
  system_prompt: "be concise"
providers:
  openai:
    api_key: sk-test
    models:
      - gpt-4o
      - gpt-4o-mini
  anthropic:
    api_key: an-test
    models:
      - claude-sonnet-4-20250514
aliases:
  claude: anthropic/claude-sonnet-4-20250514
  fast: openai/gpt-4o-mini
`)
	cfg, err := loadFromBytes(t, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultModel != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("DefaultModel = %q", cfg.DefaultModel)
	}
	if cfg.Defaults.Temperature != 0.3 {
		t.Errorf("Temperature = %v", cfg.Defaults.Temperature)
	}
	if cfg.Defaults.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %d", cfg.Defaults.MaxTokens)
	}
	if cfg.Defaults.SystemPrompt != "be concise" {
		t.Errorf("SystemPrompt = %q", cfg.Defaults.SystemPrompt)
	}
	if cfg.Providers["openai"].APIKey != "sk-test" {
		t.Errorf("openai api key = %q", cfg.Providers["openai"].APIKey)
	}
	if got := cfg.Aliases["claude"]; got != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("alias claude = %q", got)
	}
}

func TestLoadFromBytes_AppliesDefaults(t *testing.T) {
	cfg, err := loadFromBytes(t, []byte(``))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultModel != "openai/gpt-4o" {
		t.Errorf("default DefaultModel = %q", cfg.DefaultModel)
	}
	if cfg.Defaults.Temperature != 0.7 {
		t.Errorf("default Temperature = %v", cfg.Defaults.Temperature)
	}
	if cfg.Defaults.MaxTokens != 4096 {
		t.Errorf("default MaxTokens = %d", cfg.Defaults.MaxTokens)
	}
	if cfg.Defaults.Streaming == nil {
		t.Fatalf("default Streaming = nil, want non-nil pointer to true")
	}
	if !*cfg.Defaults.Streaming {
		t.Errorf("default Streaming = false, want true")
	}
	if cfg.Providers == nil {
		t.Errorf("Providers map nil")
	}
	if cfg.Aliases == nil {
		t.Errorf("Aliases map nil")
	}
}

// TestLoadFromBytes_StreamingExplicit covers all three states for the
// Defaults.Streaming field: unset (default true), explicit true, and
// explicit false. The explicit-false case is the regression from #10.
func TestLoadFromBytes_StreamingExplicit(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want bool
	}{
		{
			name: "unset defaults to true",
			yaml: `defaults: {}`,
			want: true,
		},
		{
			name: "explicit true is honored",
			yaml: "defaults:\n  streaming: true\n",
			want: true,
		},
		{
			name: "explicit false is honored",
			yaml: "defaults:\n  streaming: false\n",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadFromBytes(t, []byte(tc.yaml))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Defaults.Streaming == nil {
				t.Fatalf("Streaming = nil, want non-nil pointer")
			}
			if got := *cfg.Defaults.Streaming; got != tc.want {
				t.Errorf("Streaming = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadFromBytes_InvalidYAML(t *testing.T) {
	_, err := loadFromBytes(t, []byte("default_model: [unterminated"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_MissingFileReturnsError(t *testing.T) {
	// Load() reads from ~/.config/tchat/config.yaml — override $HOME to a
	// temp dir to make sure the file is missing, no matter what the user
	// running the tests has in their real home.
	t.Setenv("HOME", t.TempDir())
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveModel(t *testing.T) {
	cfg := &Config{
		Providers: map[string]provider.ProviderConfig{
			"openai": {
				APIKey: "sk",
				Models: []string{"gpt-4o", "gpt-4o-mini"},
			},
			"anthropic": {
				APIKey: "ak",
				Models: []string{"claude-sonnet-4-20250514"},
			},
		},
		Aliases: map[string]string{
			"claude": "anthropic/claude-sonnet-4-20250514",
			"fast":   "openai/gpt-4o-mini",
			// An alias pointing at a bare model name exercises the alias-
			// rewrite + bare-model search path together.
			"mini": "gpt-4o-mini",
		},
	}

	cases := []struct {
		name      string
		input     string
		wantProv  string
		wantModel string
		wantErr   bool
	}{
		{"alias to provider/model", "claude", "anthropic", "claude-sonnet-4-20250514", false},
		{"alias to another alias target", "fast", "openai", "gpt-4o-mini", false},
		{"alias to bare model", "mini", "openai", "gpt-4o-mini", false},
		{"provider/model direct", "openai/gpt-4o", "openai", "gpt-4o", false},
		{"bare model match", "gpt-4o", "openai", "gpt-4o", false},
		{"unknown provider in provider/model", "mystery/foo", "", "", true},
		{"completely unknown", "no-such-model", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov, model, err := cfg.ResolveModel(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got (%q, %q)", prov, model)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if prov != tc.wantProv || model != tc.wantModel {
				t.Errorf("got (%q, %q), want (%q, %q)", prov, model, tc.wantProv, tc.wantModel)
			}
		})
	}
}

func TestResolveModel_EmptyConfig(t *testing.T) {
	cfg := &Config{
		Providers: map[string]provider.ProviderConfig{},
		Aliases:   map[string]string{},
	}
	if _, _, err := cfg.ResolveModel("anything"); err == nil {
		t.Fatal("expected error on empty config")
	}
}

func TestConfigPath_UsesHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home")
	got := ConfigPath()
	if !strings.Contains(got, "/.config/tchat/config.yaml") {
		t.Errorf("ConfigPath = %q, expected to contain /.config/tchat/config.yaml", got)
	}
}
