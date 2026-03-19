package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

type Executor struct {
	registry *Registry
}

func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

type ToolCall struct {
	Name   string
	Params map[string]string
}

func (e *Executor) FindFirstToolCall(response string) (ToolCall, bool) {
	scanner := bufio.NewScanner(strings.NewReader(response))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		toolRaw, ok := raw["tool"]
		if !ok {
			continue
		}
		toolName, ok := toolRaw.(string)
		if !ok || toolName == "" {
			continue
		}
		params := make(map[string]string)
		for k, v := range raw {
			if k == "tool" {
				continue
			}
			params[k] = fmt.Sprint(v)
		}
		return ToolCall{Name: toolName, Params: params}, true
	}
	return ToolCall{}, false
}

func (e *Executor) Execute(call ToolCall) (string, error) {
	tool, ok := e.registry.Get(call.Name)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
	return tool.Execute(call.Params)
}

func Truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
