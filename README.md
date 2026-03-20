# luminosity-agent

A lightweight local CLI AI agent framework in Go, built for small quantized models behind LM Studio's OpenAI-compatible API.

## Features

- **Vector memory** — markdown files in `~/.luminosity/memory/` embedded via nomic-embed-text, semantically searched per turn
- **XML tool calling** — reliable tool format for small models: `<tool>`, `<query>`, `<url>`, `<path>`, `<content>`
- **Web search** — Tavily (primary) with Brave Search fallback
- **Web fetch** — fetches and strips HTML from any URL
- **Shell tool** — runs bash commands with optional confirmation prompt
- **Note tools** — persistent read/write note storage in `~/.luminosity/notes/`
- **Streaming TUI** — live token streaming via Charm (bubbletea + lipgloss)
- **Legacy flat memory** — `/remember` command for manual fact curation
- **Context budgeting** — strict token budget management with auto-summarization

## Requirements

- Go 1.24+
- LM Studio running with:
  - A chat model loaded (e.g. `qwen3.5-4b`)
  - `nomic-embed-text-v1.5` loaded for vector memory embeddings
- Tavily API key (free tier at tavily.com) for web search
- Brave Search API key (optional fallback)

## Setup

```bash
# 1. Clone
git clone https://github.com/ahlyx/luminosity-agent
cd luminosity-agent

# 2. Create config directory and copy example
mkdir -p ~/.luminosity/memory/
cp config.yaml.example ~/.luminosity/config.yaml

# 3. Edit config with your values
nano ~/.luminosity/config.yaml

# 4. Run
go run ./cmd
```

## Configuration

Config lives at `~/.luminosity/config.yaml` — never committed to the repo.

See `config.yaml.example` for the full structure. Key fields:

| Field | Default | Description |
|---|---|---|
| `lmstudio.base_url` | `http://localhost:1234` | LM Studio server URL |
| `lmstudio.model` | `qwen3.5-4b` | Chat model name |
| `memory.dir` | `~/.luminosity/memory/` | Vector memory markdown files |
| `memory.always_inject` | `["core.md"]` | Files always injected regardless of similarity |
| `memory.top_k` | `3` | Max vector search results injected per turn |
| `search.tavily_key` | — | Tavily API key (or set `TAVILY_API_KEY` env var) |
| `search.brave_key` | — | Brave Search API key (or set `BRAVE_API_KEY` env var) |

## Vector Memory

Luminosity uses semantic search to inject only relevant memory context per turn.

**Setup:**
```bash
mkdir -p ~/.luminosity/memory/skills
mkdir -p ~/.luminosity/memory/projects
```

Create markdown files in any subdirectory:
```
~/.luminosity/memory/
  core.md              # Always injected — identity, preferences
  skills/
    go.md              # Go conventions and skill level
    security.md        # Security research context
  projects/
    my-project.md      # Project-specific context
```

On startup Luminosity embeds all files via nomic. Each turn it searches for the top `top_k` most relevant files and injects them alongside `core.md`.

**Slash commands for memory:**
- `/memory` — show all loaded chunks and legacy facts
- `/reload` — re-embed files after editing them

## Tool Calling

Tools use XML tags on their own lines:

```
<tool>web_search</tool>
<query>OT ICS security vulnerabilities 2024</query>

<tool>web_fetch</tool>
<url>https://example.com/article</url>

<tool>write_note</tool>
<path>notes/research.md</path>
<content>content here</content>

<tool>read_note</tool>
<path>notes/research.md</path>

<tool>shell</tool>
<command>ls -la</command>
```

## Trust Mode

The `shell` tool prompts for confirmation before executing commands by default.

```bash
# Skip confirmation prompts
go run ./cmd -trust

# Or set in config
tools:
  trust_mode: true
```

## Slash Commands

| Command | Description |
|---|---|
| `/help` | Show tools and descriptions |
| `/tools` | Show tool schemas |
| `/memory` | Show vector chunks and legacy facts |
| `/reload` | Re-embed memory files after editing |
| `/remember` | Multiline manual fact curation |
| `/clear` | Clear conversation history |
| `/reset` | Clear history and wipe legacy facts |
| `/quit` | Save and exit |

## Architecture

```
cmd/main.go                     — entrypoint, wires all components
config/config.go                — YAML config loader
internal/
  agent/
    headless.go                 — main agent loop, tool execution, memory injection
    context.go                  — token budget management
  client/
    lmstudio.go                 — LM Studio chat streaming + embeddings client
  memory/
    vector.go                   — vector store: embed, search, inject
    memory.go                   — legacy flat facts manager
    store.go                    — JSON persistence
  prompt/
    builder.go                  — system prompt
  tools/
    executor.go                 — XML tool call parser
    registry.go                 — tool registry
    builtin/
      web_search.go             — Tavily + Brave Search
      web_fetch.go              — HTML fetch and strip
      write_note.go             — note persistence
      read_note.go              — note retrieval
      shell.go                  — shell execution
  tui/
    tui.go                      — Charm TUI with live streaming
```