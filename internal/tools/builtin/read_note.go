package builtin

import (
	"fmt"
	"os"

	"github.com/ahlyx/luminosity-agent/internal/tools"
)

type ReadNoteTool struct{}

func (t ReadNoteTool) Name() string        { return "read_note" }
func (t ReadNoteTool) Description() string { return "Reads a named note from local storage." }
func (t ReadNoteTool) Schema() string {
	return `{"tool":"read_note","name":"todo"}`
}

func (t ReadNoteTool) Execute(params map[string]string) (string, error) {
	name := sanitizeName(params["name"])
	if name == "" {
		return "missing parameter: name", nil
	}
	path, err := notePath(name)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Note '%s' not found.", name), nil
		}
		return "Error: " + err.Error(), nil
	}
	return tools.Truncate(string(b), 500), nil
}
