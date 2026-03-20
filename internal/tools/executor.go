package tools

import (
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

// xmlTag extracts the content of a single XML tag from s.
// Returns ("", false) if the tag is not present.
func xmlTag(s, tag string) (string, bool) {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(s, open)
	if start == -1 {
		return "", false
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return "", false
	}
	return strings.TrimSpace(s[start : start+end]), true
}

// FindFirstToolCall scans response for the first XML tool call block.
//
// Supported formats:
//
//	<tool>web_fetch</tool>
//	<url>https://example.com</url>
//
//	<tool>write_note</tool>
//	<path>/workspace/notes/foo.md</path>
//	<content>some content here</content>
//
//	<tool>read_note</tool>
//	<path>/workspace/notes/foo.md</path>
//
//	<tool>shell</tool>
//	<command>ls -la</command>
func (e *Executor) FindFirstToolCall(response string) (ToolCall, bool) {
	toolName, ok := xmlTag(response, "tool")
	if !ok || toolName == "" {
		return ToolCall{}, false
	}

	params := make(map[string]string)

	paramTags := []string{"url", "query", "path", "content", "command", "name"}
	for _, tag := range paramTags {
		if val, found := xmlTag(response, tag); found {
			params[tag] = val
		}
	}

	return ToolCall{Name: toolName, Params: params}, true
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