package builtin

import (
	"fmt"
	"os"

	"github.com/ahlyx/luminosity-agent/internal/tools"
)

type ReadFileTool struct{}

func (t ReadFileTool) Name() string        { return "read_file" }
func (t ReadFileTool) Description() string { return "Read any file from an absolute path on the local filesystem." }
func (t ReadFileTool) Schema() string {
	return "<tool>read_file</tool>\n<path>/absolute/path/to/file.json</path>"
}

func (t ReadFileTool) Execute(params map[string]string) (string, error) {
	path := params["path"]
	if path == "" {
		return "missing parameter: path", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File not found: %s", path), nil
		}
		return "Error: " + err.Error(), nil
	}
	return tools.Truncate(string(b), 12000), nil
}
