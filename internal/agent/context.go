package agent

import (
	"strings"

	"github.com/ahlyx/luminosity-agent/internal/client"
	"github.com/ahlyx/luminosity-agent/internal/tools"
)

type ContextConfig struct {
	MaxTokens       int
	SystemBudget    int
	MemoryBudget    int
	ResponseReserve int
}

type ContextManager struct {
	cfg ContextConfig
}

func NewContextManager(cfg ContextConfig) *ContextManager {
	return &ContextManager{cfg: cfg}
}

func (m *ContextManager) BuildMessages(system string, memoryMsg string, history []client.Message) []client.Message {
	trimmedMemory := trimToBudget(memoryMsg, m.cfg.MemoryBudget)
	messages := []client.Message{{Role: "system", Content: trimToBudget(system, m.cfg.SystemBudget)}}
	if strings.TrimSpace(trimmedMemory) != "" {
		messages = append(messages, client.Message{Role: "user", Content: trimmedMemory})
	}
	messages = append(messages, history...)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			msg := messages[i]
			messages[i] = msg
			break
		}
	}
	return messages
}

func (m *ContextManager) EnforceHistoryBudget(history []client.Message, autoSummarize bool, summarizeFn func([]client.Message) string, setSummary func(string)) []client.Message {
	historyBudget := m.cfg.MaxTokens - m.cfg.SystemBudget - m.cfg.MemoryBudget - m.cfg.ResponseReserve
	if historyBudget < 500 {
		historyBudget = 500
	}
	if estimateMessagesTokens(history) <= historyBudget {
		return history
	}

	preserve := 4
	if len(history) < preserve {
		preserve = len(history)
	}
	older := history[:len(history)-preserve]
	newest := history[len(history)-preserve:]

	if autoSummarize && len(older) > 0 && summarizeFn != nil {
		summary := strings.TrimSpace(summarizeFn(older))
		if summary != "" && setSummary != nil {
			setSummary(tools.Truncate(summary, 1500))
		}
	}

	for estimateMessagesTokens(newest) > historyBudget && len(newest) > 0 {
		newest = newest[1:]
	}

	return newest
}

func trimToBudget(text string, budget int) string {
	if budget <= 0 {
		return ""
	}
	if estimateTokens(text) <= budget {
		return text
	}
	words := strings.Fields(text)
	for len(words) > 0 {
		words = words[:len(words)-1]
		candidate := strings.Join(words, " ")
		if estimateTokens(candidate) <= budget {
			return candidate
		}
	}
	return ""
}

func estimateTokens(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return int(float64(len(strings.Fields(text)))*1.3) + 1
}

func estimateMessagesTokens(messages []client.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content) + 4
	}
	return total
}
