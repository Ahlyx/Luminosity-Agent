# luminosity-agent

A local CLI AI agent framework in Go, built for small quantized models via LM Studio. Combines semantic vector memory, multi-step tool chaining, and RAG ingestion to get more out of small models.

## Features

- **Vector RAG memory** — markdown files in `~/.luminosity/memory/` embedded via nomic-embed-text, semantically searched and injected per turn
- **save_memory tool** — ingest arbitrary content (web pages, articles, research) into the vector store mid-conversation with automatic chunking and cosine deduplication (threshold 0.87)
- **Multi-step tool chaining** — agent loops up to 8 tool calls per turn, with a guaranteed synthesis pass if the loop exhausts without producing prose
- **XML tool calling** — reliable format for small models: `<tool>`, `<query>`, `<url>`, `<path>`, `<content>`
- **Web search** — Tavily (primary) with Brave Search fallback
- **Web fetch** — fetches and strips HTML from any URL, decodes HTML entities
- **Shell tool** — runs bash commands with optional confirmation prompt
- **Note tools** — persistent read/write note storage in `~/.luminosity/notes/`
- **Streaming TUI** — live token streaming via Charm (bubbletea + lipgloss), think block stripping
- **Context budgeting** — token budget management with auto-summarization of old history
- **Legacy flat memory** — `/remember` command for manual fact curation via LLM merge

## Requirements

- Go 1.24+
- LM Studio running with:
  - A chat model loaded (e.g. `qwen3.5-4b` or `qwen3.5-9b`)
  - `nomic-embed-text-v1.5` loaded for vector memory embeddings
- Tavily API key (free tier at tavily.com) for web search
- Brave Search API key (optional fallback)

## Setup
```bash
# 1. Clone
git clone https://github.com/ahlyx/luminosity-agent
cd luminosity-agent

# 2. Create config directory
mkdir -p ~/.luminosity/memory/

# 3. Copy and edit config
cp config.yaml.example ~/.luminosity/config.yaml
nano ~/.luminosity/config.yaml

# 4. Run
go run ./cmd

# Optional: enable trust mode to skip shell confirmation prompts
go run ./cmd -trust
```

## Configuration

Config lives at `~/.luminosity/config.yaml` — never committed to the repo.

| Field | Default | Description |
|---|---|---|
| `lmstudio.base_url` | `http://localhost:1234` | LM Studio server URL |
| `lmstudio.model` | `qwen3.5-4b` | Chat model identifier |
| `context.max_tokens` | `16384` | Total context window budget |
| `memory.dir` | `~/.luminosity/memory/` | Vector memory root directory |
| `memory.always_inject` | `["core.md"]` | Files always injected regardless of similarity |
| `memory.top_k` | `3` | Max vector search results injected per turn |
| `search.tavily_key` | — | Tavily API key (or `TAVILY_API_KEY` env var) |
| `search.brave_key` | — | Brave Search API key (or `BRAVE_API_KEY` env var) |

## Vector Memory

On startup, Luminosity embeds all markdown files under `~/.luminosity/memory/` via nomic-embed-text. Each turn it searches for the top `top_k` most semantically relevant chunks and injects them into context alongside any `always_inject` files.

**Directory structure:**
```
~/.luminosity/memory/
  core.md              # Always injected — identity, preferences, context
  skills/
    go.md
    security.md
  projects/
    my-project.md
  ingested/            # Auto-created by save_memory tool
    *.md
```

The `ingested/` subdirectory is managed automatically. When the agent calls `save_memory`, content is chunked, embedded, deduplicated, and written here so it persists across restarts.

## Tools
```
web_search     Search the web (Tavily + Brave fallback)
web_fetch      Fetch a URL and return plain text
save_memory    Chunk and embed content into vector memory
write_note     Write a note to ~/.luminosity/notes/
read_note      Read a note from ~/.luminosity/notes/
shell          Run a bash command
```

**Tool call format** (XML, one tool per response):
```
<tool>web_search</tool>
<query>ICS OT vulnerabilities 2025</query>

<tool>web_fetch</tool>
<url>https://example.com/article</url>

<tool>save_memory</tool>
<path>source-label</path>
<content>text to remember</content>

<tool>write_note</tool>
<path>notes/research.md</path>
<content>content here</content>

<tool>shell</tool>
<command>ls -la</command>
```

## Slash Commands

| Command | Description |
|---|---|
| `/help` | List tools with descriptions |
| `/tools` | List tools with schemas |
| `/memory` | Show vector chunks and legacy facts |
| `/reload` | Re-embed memory files after editing |
| `/remember` | Multiline manual fact curation (LLM-merged) |
| `/clear` | Clear conversation history |
| `/reset` | Clear history and wipe legacy facts |
| `/quit` | Save and exit |

## Architecture
```
cmd/main.go                     — entrypoint, wires all components
config/config.go                — YAML config loader with env var overrides
internal/
  agent/
    headless.go                 — agent loop: tool chaining, synthesis pass, memory injection
    context.go                  — token budget management, history trimming, auto-summarization
  client/
    lmstudio.go                 — LM Studio streaming chat + embeddings, think block stripping
  memory/
    vector.go                   — vector store: embed, search, inject, persist, deduplication
    chunker.go                  — overlapping word-count chunker for RAG ingestion
    memory.go                   — legacy flat facts manager
    store.go                    — JSON persistence
  prompt/
    builder.go                  — system prompt
  tools/
    executor.go                 — XML tool call parser
    registry.go                 — tool registry
    builtin/
      web_search.go             — Tavily + Brave Search
      web_fetch.go              — HTML fetch, strip, entity decode
      save_memory.go            — RAG ingestion with deduplication
      write_note.go             — note persistence
      read_note.go              — note retrieval
      shell.go                  — shell execution with trust mode
  tui/
    tui.go                      — Charm TUI: streaming, tool blocks, banner, viewport
```