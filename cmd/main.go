package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ahlyx/luminosity-agent/config"
	"github.com/ahlyx/luminosity-agent/internal/agent"
	"github.com/ahlyx/luminosity-agent/internal/client"
	"github.com/ahlyx/luminosity-agent/internal/memory"
	"github.com/ahlyx/luminosity-agent/internal/tools"
	"github.com/ahlyx/luminosity-agent/internal/tools/builtin"
	"github.com/ahlyx/luminosity-agent/internal/tui"
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

	registry := tools.NewRegistry()
	registry.Register(builtin.WebFetchTool{})
	registry.Register(builtin.WriteNoteTool{})
	registry.Register(builtin.ReadNoteTool{})
	registry.Register(&builtin.ShellTool{TrustMode: trustMode})

	inputCh := make(chan string, 1)

	m := tui.New(inputCh)
	p := tea.NewProgram(m, tea.WithAltScreen())

	go runAgent(cfg, lm, mem, registry, trustMode, inputCh, p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runAgent(
	cfg config.Config,
	lm *client.LMStudioClient,
	mem *memory.Manager,
	registry *tools.Registry,
	trustMode bool,
	inputCh <-chan string,
	p *tea.Program,
) {
	corrupted, err := mem.Load()
	if err != nil {
		p.Send(tui.AgentMsg{Kind: tui.KindError, Text: "Failed to load memory: " + err.Error()})
		return
	}
	if corrupted {
		p.Send(tui.AgentMsg{Kind: tui.KindSystem, Text: "Memory file corrupted, starting fresh."})
	}

	a := agent.NewHeadless(cfg, lm, mem, registry, trustMode, func(kind tui.MsgKind, text string) {
		p.Send(tui.AgentMsg{Kind: kind, Text: text})
	})

	for input := range inputCh {
		a.Handle(input)
	}
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}