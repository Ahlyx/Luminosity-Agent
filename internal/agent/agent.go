package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ahlyx/luminosity-agent/config"
	"github.com/ahlyx/luminosity-agent/internal/client"
	"github.com/ahlyx/luminosity-agent/internal/memory"
	"github.com/ahlyx/luminosity-agent/internal/prompt"
	"github.com/ahlyx/luminosity-agent/internal/tools"
	"github.com/chzyer/readline"
)

type Agent struct {
	cfg        config.Config
	lm         *client.LMStudioClient
	memory     *memory.Manager
	ctxMgr     *ContextManager
	registry   *tools.Registry
	executor   *tools.Executor
	history    []client.Message
	rl         *readline.Instance
	trustMode  bool
	systemText string
}

func New(cfg config.Config, lm *client.LMStudioClient, mem *memory.Manager, registry *tools.Registry, rl *readline.Instance, trustMode bool) *Agent {
	ctxMgr := NewContextManager(ContextConfig{
		MaxTokens:       cfg.Context.MaxTokens,
		SystemBudget:    cfg.Context.SystemBudget,
		MemoryBudget:    cfg.Context.MemoryBudget,
		ResponseReserve: cfg.Context.ResponseReserve,
	})
	return &Agent{
		cfg:        cfg,
		lm:         lm,
		memory:     mem,
		ctxMgr:     ctxMgr,
		registry:   registry,
		executor:   tools.NewExecutor(registry),
		rl:         rl,
		trustMode:  trustMode,
		systemText: prompt.BuildSystemPrompt(),
	}
}

func (a *Agent) Run() error {
	corrupted, err := a.memory.Load()
	if err != nil {
		return err
	}
	if corrupted {
		fmt.Println("Memory file corrupted, starting fresh.")
	}

	for {
		line, err := a.rl.Readline()
		if err != nil {
			return a.memory.Save()
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			quit, err := a.handleSlash(line)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if quit {
				return a.memory.Save()
			}
			continue
		}
		if err := a.handleUserMessage(line); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

func (a *Agent) handleSlash(line string) (bool, error) {
	switch line {
	case "/help":
		for _, t := range a.registry.List() {
			fmt.Printf("- %s: %s\n", t.Name(), t.Description())
		}
		return false, nil
	case "/tools":
		for _, t := range a.registry.List() {
			fmt.Printf("- %s %s\n", t.Name(), t.Schema())
		}
		return false, nil
	case "/memory":
		fmt.Println("Facts:")
		for i, fact := range a.memory.Facts() {
			fmt.Printf("%d. %s\n", i+1, fact)
		}
		fmt.Printf("Summary: %s\n", a.memory.Summary())
		return false, nil
	case "/clear":
		a.history = nil
		fmt.Println("Conversation history cleared.")
		return false, nil
	case "/reset":
		a.history = nil
		if err := a.memory.Reset(); err != nil {
			return false, err
		}
		fmt.Println("Conversation and memory reset.")
		return false, nil
	case "/remember":
		return false, a.runRemember()
	case "/quit":
		return true, nil
	default:
		fmt.Println("Unknown command. Use /help.")
		return false, nil
	}
}

func (a *Agent) handleUserMessage(input string) error {
	a.history = append(a.history, client.Message{Role: "user", Content: input})
	a.history = a.ctxMgr.EnforceHistoryBudget(a.history, a.cfg.Memory.AutoSummarize, a.summarizeTurns, a.memory.SetSummary)

	messages := a.ctxMgr.BuildMessages(a.systemText, a.memory.InjectionMessage(), a.history)

	var printed bool
	resp, err := a.lm.StreamChat(messages, a.cfg.Context.ResponseReserve, func(tok string) {
		printed = true
		fmt.Print(tok)
	})
	if printed {
		fmt.Println()
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	a.history = append(a.history, client.Message{Role: "assistant", Content: resp})
	if err := a.memory.Save(); err != nil {
		return err
	}

	call, ok := a.executor.FindFirstToolCall(resp)
	if !ok {
		return nil
	}

	out, execErr := a.executor.Execute(call)
	if execErr != nil {
		out = "Error: " + execErr.Error()
	}
	out = tools.Truncate(out, 500)
	toolResult := map[string]string{"tool_result": call.Name, "output": out}
	b, _ := json.Marshal(toolResult)
	msg := string(b)
	a.history = append(a.history, client.Message{Role: "user", Content: msg})
	fmt.Printf("%s\n", msg)
	return a.memory.Save()
}

func (a *Agent) summarizeTurns(turns []client.Message) string {
	if len(turns) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range turns {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(t.Role)
		b.WriteString(": ")
		b.WriteString(tools.Truncate(t.Content, 300))
		b.WriteString("\n")
	}
	messages := []client.Message{
		{Role: "system", Content: "Summarize the conversation briefly in plain text for memory. Keep under 120 words."},
		{Role: "user", Content: b.String()},
	}
	resp, err := a.lm.StreamChat(messages, 220, nil)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resp)
}

func (a *Agent) runRemember() error {
	fmt.Println("Enter facts (blank line to finish):")
	lines := make([]string, 0, 8)
	for {
		line, err := a.rl.Readline()
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		fmt.Println("No new facts provided.")
		return nil
	}
	fmt.Println("Processing memory...")

	existing := a.memory.Facts()
	var promptBuilder strings.Builder
	promptBuilder.WriteString("You are a memory curator. Here are the current memory facts:\n")
	if len(existing) == 0 {
		promptBuilder.WriteString("(none)\n")
	} else {
		for i, fact := range existing {
			promptBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
		}
	}
	promptBuilder.WriteString("\nThe user wants to add the following new information:\n")
	promptBuilder.WriteString(strings.Join(lines, "\n"))
	promptBuilder.WriteString("\n\nYour task:\n")
	promptBuilder.WriteString("- Merge the new information with existing facts intelligently.\n")
	promptBuilder.WriteString("- Combine or update related facts (e.g. if a new interest is mentioned alongside an existing interest, keep both).\n")
	promptBuilder.WriteString("- Do not replace facts unless the new information explicitly contradicts and supersedes them.\n")
	promptBuilder.WriteString("- After merging, check the full facts list for contradictions. If you find a contradiction you cannot resolve, output a CONFLICT line: CONFLICT: <description of conflict>.\n")
	promptBuilder.WriteString("- Output the final merged facts list as a JSON array of strings, one fact per string, each under 20 words.\n")
	promptBuilder.WriteString("- Output ONLY the JSON array (and any CONFLICT lines before it). No explanation.")

	messages := []client.Message{{Role: "user", Content: promptBuilder.String()}}
	resp, err := a.lm.StreamChat(messages, 500, nil)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CONFLICT:") {
			fmt.Printf("[CONFLICT] %s\n", strings.TrimSpace(strings.TrimPrefix(line, "CONFLICT:")))
		}
	}

	factsJSON := extractJSONArray(resp)
	if factsJSON == "" {
		return fmt.Errorf("failed to parse curated facts from model")
	}

	var facts []string
	if err := json.Unmarshal([]byte(factsJSON), &facts); err != nil {
		return fmt.Errorf("failed to decode facts: %w", err)
	}

	a.memory.SetFacts(facts)
	if err := a.memory.Save(); err != nil {
		return err
	}
	fmt.Printf("Memory updated. %d facts stored.\n", len(a.memory.Facts()))
	return nil
}

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}
