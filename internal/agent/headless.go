package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ahlyx/luminosity-agent/config"
	"github.com/ahlyx/luminosity-agent/internal/client"
	"github.com/ahlyx/luminosity-agent/internal/memory"
	"github.com/ahlyx/luminosity-agent/internal/prompt"
	"github.com/ahlyx/luminosity-agent/internal/tools"
	"github.com/ahlyx/luminosity-agent/internal/tui"
)

// OutputFn is the callback the headless agent uses to send output to the TUI.
type OutputFn func(kind tui.MsgKind, text string)

type HeadlessAgent struct {
	cfg        config.Config
	lm         *client.LMStudioClient
	memory     *memory.Manager
	ctxMgr     *ContextManager
	registry   *tools.Registry
	executor   *tools.Executor
	history    []client.Message
	trustMode  bool
	systemText string
	output     OutputFn
}

func NewHeadless(
	cfg config.Config,
	lm *client.LMStudioClient,
	mem *memory.Manager,
	registry *tools.Registry,
	trustMode bool,
	output OutputFn,
) *HeadlessAgent {
	ctxMgr := NewContextManager(ContextConfig{
		MaxTokens:       cfg.Context.MaxTokens,
		SystemBudget:    cfg.Context.SystemBudget,
		MemoryBudget:    cfg.Context.MemoryBudget,
		ResponseReserve: cfg.Context.ResponseReserve,
	})
	return &HeadlessAgent{
		cfg:        cfg,
		lm:         lm,
		memory:     mem,
		ctxMgr:     ctxMgr,
		registry:   registry,
		executor:   tools.NewExecutor(registry),
		trustMode:  trustMode,
		systemText: prompt.BuildSystemPrompt(),
		output:     output,
	}
}

// Handle processes a single input line from the TUI.
func (a *HeadlessAgent) Handle(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	if strings.HasPrefix(input, "/") {
		quit, err := a.handleSlash(input)
		if err != nil {
			a.output(tui.KindError, err.Error())
		}
		if quit {
			a.memory.Save()
		}
		return
	}
	if err := a.handleUserMessage(input); err != nil {
		a.output(tui.KindError, err.Error())
	}
}

func (a *HeadlessAgent) handleSlash(line string) (bool, error) {
	if strings.HasPrefix(line, "/remember") {
		return false, a.runRemember()
	}
	switch line {
	case "/help":
		var sb strings.Builder
		for _, t := range a.registry.List() {
			sb.WriteString(fmt.Sprintf("%-14s %s\n", t.Name(), t.Description()))
		}
		a.output(tui.KindSystem, strings.TrimRight(sb.String(), "\n"))
		return false, nil

	case "/tools":
		var sb strings.Builder
		for _, t := range a.registry.List() {
			sb.WriteString(fmt.Sprintf("%-14s %s\n", t.Name(), t.Schema()))
		}
		a.output(tui.KindSystem, strings.TrimRight(sb.String(), "\n"))
		return false, nil

	case "/memory":
		var sb strings.Builder
		sb.WriteString("Facts:\n")
		for i, fact := range a.memory.Facts() {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
		}
		summary := a.memory.Summary()
		if summary == "" {
			summary = "none"
		}
		sb.WriteString("Summary: " + summary)
		a.output(tui.KindSystem, sb.String())
		return false, nil

	case "/clear":
		a.history = nil
		a.output(tui.KindSystem, "Conversation history cleared.")
		return false, nil

	case "/reset":
		a.history = nil
		if err := a.memory.Reset(); err != nil {
			return false, err
		}
		a.output(tui.KindSystem, "Conversation and memory reset.")
		return false, nil

	case "/quit":
		return true, nil

	default:
		a.output(tui.KindSystem, "Unknown command. Use /help.")
		return false, nil
	}
}

func (a *HeadlessAgent) handleUserMessage(input string) error {
	a.history = append(a.history, client.Message{Role: "user", Content: input})
	a.history = a.ctxMgr.EnforceHistoryBudget(
		a.history,
		a.cfg.Memory.AutoSummarize,
		a.summarizeTurns,
		a.memory.SetSummary,
	)

	messages := a.ctxMgr.BuildMessages(a.systemText, a.memory.InjectionMessage(), a.history)

	// Accumulate streaming tokens into a single assistant message
	var buf strings.Builder
	resp, err := a.lm.StreamChat(messages, a.cfg.Context.ResponseReserve, func(tok string) {
		buf.WriteString(tok)
	})

	// Send the full response as one message (avoids flickering in TUI)
	if buf.Len() > 0 {
		a.output(tui.KindAssistant, strings.TrimSpace(buf.String()))
	}
	a.output(tui.KindThinkingStop, "")

	if err != nil {
		a.output(tui.KindError, err.Error())
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
	a.output(tui.KindTool, fmt.Sprintf("%s → %s", call.Name, out))
	return a.memory.Save()
}

func (a *HeadlessAgent) summarizeTurns(turns []client.Message) string {
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

func (a *HeadlessAgent) runRemember() error {
	a.output(tui.KindSystem, "Enter facts (blank line to finish):")
	lines := make([]string, 0, 8)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		a.output(tui.KindSystem, "No new facts provided.")
		return nil
	}
	a.output(tui.KindSystem, "Processing memory...")

	existing := a.memory.Facts()
	var pb strings.Builder
	pb.WriteString("You are a memory curator. Write all facts in third person (e.g 'user's name is x' not 'my name is x'). Here are the current memory facts:\n")
	if len(existing) == 0 {
		pb.WriteString("(none)\n")
	} else {
		for i, fact := range existing {
			pb.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
		}
	}
	pb.WriteString("\nThe user wants to add the following new information:\n")
	pb.WriteString(strings.Join(lines, "\n"))
	pb.WriteString("\n\nYour task:\n")
	pb.WriteString("- Merge the new information with existing facts intelligently.\n")
	pb.WriteString("- Combine or update related facts (e.g. if a new interest is mentioned alongside an existing interest, keep both).\n")
	pb.WriteString("- Do not replace facts unless the new information explicitly contradicts and supersedes them.\n")
	pb.WriteString("- After merging, check the full facts list for contradictions. If you find a contradiction you cannot resolve, output a CONFLICT line: CONFLICT: <description of conflict>.\n")
	pb.WriteString("- Output the final merged facts list as a JSON array of strings, one fact per string, each under 20 words.\n")
	pb.WriteString("- Output ONLY the JSON array (and any CONFLICT lines before it). No explanation.")

	messages := []client.Message{{Role: "user", Content: pb.String()}}
	resp, err := a.lm.StreamChat(messages, 4096, nil)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CONFLICT:") {
			a.output(tui.KindSystem, "[CONFLICT] "+strings.TrimSpace(strings.TrimPrefix(line, "CONFLICT:")))
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
	a.output(tui.KindSystem, fmt.Sprintf("Memory updated. %d facts stored.", len(a.memory.Facts())))
	return nil
}