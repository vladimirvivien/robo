# 🤖 robo

An AI assistant designed for the shell in your terminal.

`robo` uses an on-device small language model (SLM) with cloud frontier models to generate shell commands from prompts and interact with the OS. `robo` also supports multi-turn REPL sessions and Unix pipe composability.

---

## Key Capabilities

* **Dual-Engine Architecture:** Runs quantized models locally via `litertlm-go` (Gemma 4 E4B) for private, zero-latency inference, and connects to cloud frontier models (Google Gemini, Anthropic Claude, OpenAI) via Genkit.
* **Hot-Start Daemon (`robod`):** Hosts the local model in memory for sub-50ms response latency, auto-spawns on demand, and shuts down after 15 minutes of inactivity.
* **Intelligent Routing:** Automatically chooses between local and cloud engines based on prompt complexity, token count (> 4K tokens), and failure escalation.
* **Shell Assistant (`robo do`):** Synthesizes shell commands from natural language, with interactive review (`[Run] [Edit] [Cancel]`) and typed confirmation guards for destructive operations.
* **Interactive REPL (`robo chat`):** Multi-turn terminal interface with Lipgloss cards, streaming Markdown, and pure-Go SQLite conversation persistence.
* **Unix Shell Composability:** Automatically formats for human viewing in interactive TTYs, while emitting raw unformatted streams when piped to downstream tools.

---

## Installation & Build

### Prerequisites
* Go 1.24+ (or Go 1.26+)
* Make

### Build from Source
```bash
# Build size-optimized binary (stripped with -s -w -trimpath)
make build

# Output binary is located at bin/robo (or bin/robo.exe on Windows)
./bin/robo version
```
---

## Quickstart Walkthrough

### 1. Minimal Quickstart (Zero Configuration)
Robo works out of the box with intelligent defaults. When you run robo for the first time, it will walk you through setting up a model for local inference. Or you can follow the directions below to customize your configuration in file `~/.config/robo/config.yaml`.

#### On-Device Local Inference
To run strictly on-device using local GPU/CPU hardware:

```yaml
llm:
  local:
    enabled: true                     # Uses Gemma 4 E4B locally via LiteRT-LM (default)
```

#### Setup a Cloud Model
You can enable a cloud model by configuring an entry similar to the following:

```yaml
llm:
  cloud:
    provider: "googleai"              # "googleai", "anthropic", or "openai"
    model: "googleai/gemini-2.5-flash"
```
---

### 2. Direct Prompts
Ask questions directly from your terminal. Robo automatically decides whether to answer on-device or escalate to the cloud:

```bash
# Auto-routed query
robo "Explain how Go channels work under the hood"

# Force local-only execution
robo -l "Write a regex to validate IPv4 addresses"

# Force cloud-only execution for large-context tasks
robo -c "Review this architecture pattern for distributed consensus"
```

---

### 3. Shell Command Assistant (`robo do`)
Generate executable shell commands tailored to your active OS and shell:

```bash
robo do "find all files modified in the last 24 hours over 100MB"
```

Robo displays the proposed command in an interactive card:
* `[Run]` — Executes the command immediately.
* `[Edit]` — Opens the command in your line editor.
* `[Cancel]` — Aborts execution.

For potentially destructive commands (`rm -rf`, `DROP TABLE`, `kill -9`), Robo requires typing confirmation (`yes-delete`) before running.

---

### 4. Piped Input & Shell Diagnostics
Pipe file contents, logs, or command errors directly into Robo:

```bash
# Analyze test failures
go test ./... 2>&1 | robo "why did these tests fail and how do i fix them?"

# Explain a source file
cat main.go | robo "summarize the entrypoint logic"

# Format output as raw code or JSON for scripting
robo "Generate a JSON schema for a user profile" -o json
```

---

### 5. Interactive Multi-Turn REPL (`robo chat`)
Launch the interactive conversational console:

```bash
robo chat
```

* Multi-turn conversations are automatically stored in `~/.config/robo/history.db`.
* In-session commands:
  * `/local` — Switch active engine to local on-device SLM.
  * `/cloud` — Switch active engine to cloud LLM.
  * `/clear` — Clear current session history.
  * `/save <file>` — Export conversation transcript.
  * `/exit` or `Ctrl+D` — Exit REPL.

---

### 6. Daemon Management (`robo daemon`)
`robod` launches automatically in the background when needed, but can also be controlled manually:

```bash
# Check daemon status and model load state
robo daemon status

# Start daemon explicitly
robo daemon start

# Stop background daemon and free memory
robo daemon stop
```

---

## CLI Command Reference

| Command | Shorthand / Flags | Description |
|---|---|---|
| `robo [prompt]` | `-l`, `-c`, `-o <format>` | Query Robo directly or via piped stdin. |
| `robo do [intent]` | `--shell <name>` | Synthesize and execute a shell command with confirmation. |
| `robo chat` | `--session <id>` | Launch the interactive multi-turn REPL. |
| `robo daemon` | `start`, `stop`, `status` | Manage the background `robod` model server. |
| `robo version` | `-o json`, `-o plain` | Print build version, commit SHA, and platform information. |

### Global Output Formats (`-o` / `--output`)
* `markdown` (default in TTY) — Rich styled Markdown rendered via Glamour.
* `plain` — Plain text output without ANSI color escape sequences.
* `json` — Structured JSON payload.
* `code` — Extracts and emits only fenced code blocks.

---

## License

Apache 2.0. See `LICENSE` for details.
