# 🤖 robo

An AI assistant designed for the shell in your terminal.

`robo` uses an on-device small language model (SLM) along with cloud frontier models to generate shell commands from natural-language prompts, explain terminal diagnostics, and execute tasks directly in your active shell using your recent command history as context.

---

## Key Capabilities

* **Dual-Engine Architecture:** Runs quantized models locally via `litertlm-go` (Gemma 4 E4B) for private, zero-latency inference, and connects to cloud frontier models (Google Gemini, Anthropic Claude, OpenAI, Ollama) via native REST clients.
* **Ambient Shell Context:** Automatically reads your active OS, shell environment (Bash, Zsh, Fish, PowerShell), current working directory, and recent shell command history to contextualize prompts without manual re-typing.
* **Hot-Start Daemon (`robod`):** Hosts the local model in memory for sub-50ms response latency, auto-spawns on demand, and shuts down after 15 minutes of inactivity.
* **Intelligent Routing:** Automatically chooses between local and cloud engines based on prompt complexity, token count (> 4K tokens), and failure escalation.
* **Integrated Command Synthesis & Execution:** Translates natural language into commands with interactive review (`[Run] [Edit] [Cancel]`) and typed confirmation guards (`yes-delete`) for destructive operations.
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
Robo works out of the box with intelligent defaults. When you run robo for the first time, it will walk you through setting up a model for local inference. Or you can customize your configuration in file `~/.config/robo/config.yaml`.

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
    provider: "googleai"              # "googleai", "anthropic", "openai", or "ollama"
    model: "googleai/gemini-2.5-flash"
```
---

### 2. Command Synthesis & Interactive Execution
Ask Robo to accomplish any task in natural language. Robo synthesizes the command tailored to your active OS and shell:

```bash
robo "find all files modified in the last 24 hours over 100MB"
```

Robo displays the proposed command in an interactive card:
* `[Run]` — Executes the command immediately in your active shell.
* `[Edit]` — Opens the command in your line editor.
* `[Cancel]` — Aborts execution.

For potentially destructive commands (`rm -rf`, `DROP TABLE`, `kill -9`), Robo requires typing confirmation (`yes-delete`) before running.

#### Auto-Accept Execution
```bash
# Auto-run safe, non-destructive commands without prompt
robo -y "list listening tcp ports"

# Force local-only execution
robo -l "extract all .tar.gz archives in current directory"

# Force cloud-only execution for large-context tasks
robo -c "Review this docker-compose configuration"
```

---

### 3. Piped Input & Shell Diagnostics
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

### 4. Ambient Shell History Context
Robo automatically inspects your recent command history to understand context. For example, if you just ran a failing build or git command:

```bash
# After running a failing command:
cargo build
# error: could not find `Cargo.toml` in `/home/user/project`

# Robo automatically knows what command you just ran:
robo "how do i fix that error?"
```

---

### 5. Daemon Management (`robo daemon`)
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
| `robo [intent]` | `-y`, `-l`, `-c`, `-o <format>` | Synthesize and execute shell commands or answer terminal queries. |
| `robo daemon` | `start`, `stop`, `status` | Manage the background `robod` model server. |
| `robo version` | `-o json`, `-o plain` | Print build version, commit SHA, and platform information. |

### Global Flags
* `-y`, `--auto-accept` — Automatically execute safe, non-destructive commands without prompting.
* `--yolo-approve-all` — Auto-accept and execute all commands including destructive ones.
* `-l`, `--local-only` — Force execution on local on-device SLM.
* `-c`, `--cloud-only` — Force execution on cloud frontier model.
* `-o`, `--output <format>` — Global output format (`markdown`, `plain`, `json`, `code`).

---

## License

Apache 2.0. See `LICENSE` for details.
