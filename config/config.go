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
	Search   SearchConfig   `yaml:"search"`
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
	// Path is the legacy flat facts JSON file — kept for /remember compatibility
	Path          string `yaml:"path"`
	MaxFacts      int    `yaml:"max_facts"`
	AutoSummarize bool   `yaml:"auto_summarize"`
 
	// Dir is the root directory for vector memory markdown files
	// Defaults to ~/.luminosity/memory/
	Dir string `yaml:"dir"`
 
	// AlwaysInject is a list of memory file names always injected
	// regardless of query similarity — e.g. ["core.md"]
	AlwaysInject []string `yaml:"always_inject"`
 
	// TopK is how many vector search results to inject per turn (default 3)
	TopK int `yaml:"top_k"`

	// ChunkOnLoad is a list of file basenames to chunk rather than embed whole.
	// Useful for large domain knowledge files (e.g. "security.md").
	ChunkOnLoad []string `yaml:"chunk_on_load"`
 
	// EmbedModel is the model name for embeddings via LM Studio
	// Defaults to text-embedding-nomic-embed-text-v1.5
	EmbedModel string `yaml:"embed_model"`
}
 
type ToolsConfig struct {
	TrustMode bool `yaml:"trust_mode"`
}
 
type SearchConfig struct {
	TavilyKey string `yaml:"tavily_key"`
	BraveKey  string `yaml:"brave_key"`
}
 
func Default() Config {
	return Config{
		LMStudio: LMStudioConfig{
			BaseURL:        "http://localhost:1234",
			Model:          "qwen/qwen3.5-9b",
			TimeoutSeconds: 120,
		},
		Context: ContextConfig{
			MaxTokens:       16384,
			SystemBudget:    600,
			MemoryBudget:    2000,
			ResponseReserve: 1500,
		},
		Memory: MemoryConfig{
			Path:          "~/.luminosity/memory.json",
			MaxFacts:      50,
			AutoSummarize: true,
			Dir:           "~/.luminosity/memory/",
			AlwaysInject:  []string{"core.md"},
			TopK:          3,
			EmbedModel:    "text-embedding-nomic-embed-text-v1.5",
		},
		Tools:  ToolsConfig{TrustMode: false},
		Search: SearchConfig{TavilyKey: "", BraveKey: ""},
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
 
	// Expand ~ paths
	cfg.Memory.Path, err = ExpandPath(cfg.Memory.Path)
	if err != nil {
		return cfg, err
	}
	cfg.Memory.Dir, err = ExpandPath(cfg.Memory.Dir)
	if err != nil {
		return cfg, err
	}
 
	// Defaults
	if cfg.Memory.MaxFacts <= 0 {
		cfg.Memory.MaxFacts = 50
	}
	if cfg.Memory.TopK <= 0 {
		cfg.Memory.TopK = 3
	}
	if cfg.Memory.EmbedModel == "" {
		cfg.Memory.EmbedModel = "text-embedding-nomic-embed-text-v1.5"
	}
	if len(cfg.Memory.AlwaysInject) == 0 {
		cfg.Memory.AlwaysInject = []string{"core.md"}
	}
	if cfg.LMStudio.TimeoutSeconds <= 0 {
		cfg.LMStudio.TimeoutSeconds = 120
	}
 
	// Allow env var overrides for keys
	if cfg.Search.TavilyKey == "" {
		cfg.Search.TavilyKey = os.Getenv("TAVILY_API_KEY")
	}
	if cfg.Search.BraveKey == "" {
		cfg.Search.BraveKey = os.Getenv("BRAVE_API_KEY")
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
