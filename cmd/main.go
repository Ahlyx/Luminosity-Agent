package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ahlyx/luminosity-agent/config"
	"github.com/ahlyx/luminosity-agent/internal/agent"
	"github.com/ahlyx/luminosity-agent/internal/client"
	"github.com/ahlyx/luminosity-agent/internal/memory"
	"github.com/ahlyx/luminosity-agent/internal/tools"
	"github.com/ahlyx/luminosity-agent/internal/tools/builtin"
	"github.com/chzyer/readline"
)

func main() {
	defaultConfig := filepath.Join(userHome(), ".luminosity", "config.yaml")
	configPath := flag.String("config", defaultConfig, "Path to config.yaml")
	trustFlag := flag.Bool("trust", false, "Enable trust mode for shell tool")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	trustMode := cfg.Tools.TrustMode || *trustFlag
	lm := client.New(cfg.LMStudio.BaseURL, cfg.LMStudio.Model, cfg.LMStudio.TimeoutSeconds)
	mem := memory.NewManager(cfg.Memory.Path, cfg.Memory.MaxFacts)

	rl, err := readline.NewEx(&readline.Config{Prompt: "> "})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	registry := tools.NewRegistry()
	registry.Register(builtin.WebFetchTool{})
	registry.Register(builtin.WriteNoteTool{})
	registry.Register(builtin.ReadNoteTool{})
	registry.Register(&builtin.ShellTool{TrustMode: trustMode, Input: bufio.NewReader(os.Stdin)})

	a := agent.New(cfg, lm, mem, registry, rl, trustMode)
	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Agent exited with error: %v\n", err)
		os.Exit(1)
	}
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
