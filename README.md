# luminosity-agent

A lightweight local CLI AI agent framework in Go, tuned for small quantized models (like 4B Q6) behind LM Studio's OpenAI-compatible API.

## Features

- Persistent cross-session memory in `~/.luminosity/memory.json`
- Lean tool system with one-tool-per-turn execution
- Context manager for strict 8192-token budgeting
- Streaming LM Studio chat responses in terminal
- Slash commands for memory management and runtime controls
- `/remember` multiline memory curation flow

## Requirements

- Go 1.23+
- LM Studio running with an OpenAI-compatible local server endpoint

## Setup

1. Clone this repository.
2. Copy the example config:
   ```bash
   mkdir -p ~/.luminosity
   cp config.yaml.example ~/.luminosity/config.yaml
   ```
3. Edit `~/.luminosity/config.yaml` and set:
   - `lmstudio.base_url` to your LM Studio server URL (for example VM->Windows host IP)
   - `lmstudio.model` to your loaded model name
4. Run:
   ```bash
   go run ./cmd
   ```

## Trust mode (`-trust`)

By default, the `shell` tool asks for confirmation before command execution.

Enable trust mode to skip prompts:

```bash
go run ./cmd -trust
```

Trust mode is enabled if either:
- `-trust` is set, or
- `tools.trust_mode: true` in config

## Slash commands

- `/help` – show tools and one-line descriptions
- `/tools` – show compact JSON schemas
- `/memory` – print current facts + summary
- `/clear` – clear conversation history only
- `/reset` – clear history and wipe memory
- `/remember` – multiline memory curation mode
- `/quit` – save memory and exit

## `/remember` usage

When you run `/remember`, enter facts line by line and end with a blank line.
The agent sends a separate curation request to merge/update facts, checks for conflicts, and saves results.

Example:

```text
> /remember
Enter facts (blank line to finish):
my name is alex
i work on OT/ICS security
i am 20 years old studying at CNM

Processing memory...
[CONFLICT] Existing fact ...
Memory updated. 12 facts stored.
```

## Example session showing persistence across runs

Run 1:

```text
$ go run ./cmd
> /remember
Enter facts (blank line to finish):
my name is alex
i work on OT/ICS security

Processing memory...
Memory updated. 2 facts stored.
> /quit
```

Run 2:

```text
$ go run ./cmd
> /memory
Facts:
1. user's name is alex
2. user works on OT/ICS security
Summary: none
> /quit
```

## Notes on design

- System prompt is static and ends with `Be concise.`
- Tool outputs are always truncated to 500 chars before reinjection.
- Memory injection is added as a **user** message before the active turn.
- Summarization is isolated and triggered when history budget is exceeded.

## Dependencies

Only:
- `github.com/chzyer/readline`
- `gopkg.in/yaml.v3`

