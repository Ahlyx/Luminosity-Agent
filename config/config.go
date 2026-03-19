package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LMStudio LMStudioConfig `yaml:"lmstudio"`
	Context  ContextConfig  `yaml:"context"`
	Memory   MemoryConfig   `yaml:"memory"`
	Tools    ToolsConfig    `yaml:"tools"`
}

type LMStudioConfig struct {
	BaseURL        string `yaml:"base_url"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type ContextConfig struct {
	MaxTokens       int `yaml:"max_tokens"`
	SystemBudget    int `yaml:"system_budget"`
	MemoryBudget    int `yaml:"memory_budget"`
	ResponseReserve int `yaml:"response_reserve"`
}

type MemoryConfig struct {
	Path          string `yaml:"path"`
	MaxFacts      int    `yaml:"max_facts"`
	AutoSummarize bool   `yaml:"auto_summarize"`
}

type ToolsConfig struct {
	TrustMode bool `yaml:"trust_mode"`
}

func Default() Config {
	return Config{
		LMStudio: LMStudioConfig{
			BaseURL:        "http://192.168.189.1:1234",
			Model:          "qwen3.5-4b",
			TimeoutSeconds: 120,
		},
		Context: ContextConfig{
			MaxTokens:       8192,
			SystemBudget:    400,
			MemoryBudget:    600,
			ResponseReserve: 1024,
		},
		Memory: MemoryConfig{
			Path:          "~/.luminosity/memory.json",
			MaxFacts:      50,
			AutoSummarize: true,
		},
		Tools: ToolsConfig{TrustMode: false},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	resolved, err := ExpandPath(path)
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	cfg.Memory.Path, err = ExpandPath(cfg.Memory.Path)
	if err != nil {
		return cfg, err
	}

	if cfg.Memory.MaxFacts <= 0 {
		cfg.Memory.MaxFacts = 50
	}

	if cfg.LMStudio.TimeoutSeconds <= 0 {
		cfg.LMStudio.TimeoutSeconds = 120
	}

	return cfg, nil
}

func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Clean(path), nil
}
