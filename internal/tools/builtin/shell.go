package builtin

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
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
		return "Shell execution requires trust mode. Run Lumi with -trust flag or set tools.trust_mode: true in config.yaml", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		c = exec.CommandContext(ctx, "bash", "-c", cmd)
	}
	out, err := c.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "Command timed out.", nil
	}
	if err != nil && len(out) == 0 {
		return "Error: " + err.Error(), nil
	}
	return tools.Truncate(string(out), 500), nil
}