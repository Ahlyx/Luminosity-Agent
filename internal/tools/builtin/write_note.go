package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type WriteNoteTool struct{}

func (t WriteNoteTool) Name() string { return "write_note" }
func (t WriteNoteTool) Description() string {
	return "Writes a named note to local persistent storage."
}
func (t WriteNoteTool) Schema() string {
	return `{"tool":"write_note","name":"todo","content":"buy milk"}`
}

func (t WriteNoteTool) Execute(params map[string]string) (string, error) {
	name := sanitizeName(params["name"])
	content := params["content"]
	if name == "" {
		return "missing parameter: name", nil
	}

	path, err := notePath(name)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "Error: " + err.Error(), nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "Error: " + err.Error(), nil
	}
	return fmt.Sprintf("Note '%s' saved.", name), nil
}

func notePath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".luminosity", "notes", name+".txt"), nil
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = unsafeChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._-")
	return name
}
