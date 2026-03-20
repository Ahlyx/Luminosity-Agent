package builtin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ahlyx/luminosity-agent/internal/tools"
)

type ShellTool struct {
	TrustMode bool
}

func (t *ShellTool) Name() string        { return "shell" }
func (t *ShellTool) Description() string { return "Runs a shell command with optional confirmation." }
func (t *ShellTool) Schema() string {
	return "<tool>shell</tool>\n<command>ls -la</command>"
}

func (t *ShellTool) Execute(params map[string]string) (string, error) {
	cmd := strings.TrimSpace(params["command"])
	if cmd == "" {
		return "missing parameter: command", nil
	}

	if !t.TrustMode {
		fmt.Printf("Shell command: %s\n", cmd)
		fmt.Print("Execute? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			return "User declined execution", nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	out, err := c.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "Command timed out.", nil
	}
	if err != nil && len(out) == 0 {
		return "Error: " + err.Error(), nil
	}
	return tools.Truncate(string(out), 500), nil
}