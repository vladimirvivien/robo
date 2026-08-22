# 🤖 robo

An on-device AI system assistant for the terminal.

`robo` runs quantized Small Language Models (SLMs) locally on your hardware using LiteRT-LM to generate platform-specific shell commands, analyze terminal diagnostics, and automate developer workflows directly in your active shell.

---

## Key Capabilities

* **On-Device Local Inference:** Runs quantized models locally via LiteRT-LM (Gemma 4 2B to 12B) with hardware acceleration (GPU / WebGPU / CPU) for private, offline execution.
* **Self-Contained Asset Management (`~/.robo`):** Manages configuration, model weights, native runtime libraries, and logs directly under a single root directory.
* **Dynamic Platform Dialect Prompting:** Automatically detects your OS and active shell (PowerShell, Bash, Zsh, Fish) and constructs compact, token-efficient system prompts tailored specifically to your target environment.
* **Ambient Shell History Awareness:** Ingests recent terminal command history to contextualize requests without manual copy-pasting.
* **Background Daemon (`robod`):** Keeps local models resident in memory for fast execution, automatically starting on demand and shutting down after inactivity.
* **Hybrid Cloud Option:** Configurable optional routing to cloud frontier models (Gemini, Claude, OpenAI) for high-token or multimodal tasks.
* **Interactive Safety & Execution:** Generates executable commands with an interactive menu (`[Run]`, `[Edit]`, `[Cancel]`) and safety classification for destructive operations.

---

## Installation & Build

### Prerequisites
* Go 1.24+
* Git / jj

### Build from Source
```bash
# Clone the repository
git clone https://github.com/vladimirvivien/robo.git
cd robo

# Build the binary
go build -o bin/robo .

# Verify installation
./bin/robo version
```

---

## Quickstart

### 1. Initialize and Download Local Model
Run the setup wizard to select your local model and hardware backend:

```bash
# Interactive setup wizard
./bin/robo init

# Or initialize non-interactively with defaults (Gemma 4 2B + GPU)
./bin/robo init -y
```

You can also fetch models or runtime libraries directly using `robo get`:
```bash
# Download a specific model asset to ~/.robo/cache
./bin/robo get --model litert-community/gemma-4-E2B-it

# Download specific LiteRT-LM native runtime libraries to ~/.robo/lib
./bin/robo get --litertlm-lib v0.16.0
```

### 2. Run Your First Query
Ask `robo` to perform a task in natural language:

```bash
./bin/robo "what is the process using the most CPU"
```

`robo` synthesizes the exact platform-specific command and presents an interactive menu:
```text
╭──────────────────────────────────────────────────────────────────────────────╮
│ 🤖 Proposed Shell Command                                                    │
│                                                                              │
│    Get-Process | Sort-Object CPU -Descending | Select-Object -First 5        │
╰──────────────────────────────────────────────────────────────────────────────╯

┃ Execute command?
┃ > Run command
┃   Edit command
┃   Cancel
```

---

## Common Use Cases & Examples

### 1. System & Resource Inspection
Inspect hardware, memory, or processes using native platform tools:

```bash
# Query top resource consumers
robo "find the top 3 processes using the most memory"

# Sample system metrics over an interval
robo "wait 3 seconds and show me active network listening ports"
```

### 2. File Search & Batch Manipulation
Synthesize file searches and batch operations tailored to your shell:

```bash
# Search for files by extension and modification date
robo "find all .log files modified today and compress them into logs.zip"

# Search code repositories
robo "find all go files containing the word HandleStream"
```

### 3. Git & Developer Workflows with Ambient Context
`robo` reads your recent terminal command history to resolve contextual questions:

```bash
# Example: You run a command that encounters an issue
git push origin main
# To github.com:user/repo.git
# ! [rejected]        main -> main (fetch first)

# Ask robo to resolve it using context:
robo "how do i safely integrate remote changes without losing my work?"
```

---

## Unix Pipeline & Stream Composability

`robo` automatically detects whether `stdout` is connected to an interactive terminal or a pipeline:

```bash
# Extract raw output for scripting (-o json / -o code / -o plain)
robo "generate a json schema for a user profile" -o json | jq .

# Pipe diagnostics or file contents into robo
go test ./... 2>&1 | robo "why did this test fail and how do i fix it?"

# Automatically execute non-destructive commands without interactive prompts (-y)
robo -y "show total disk usage in current directory"
```

---

## CLI Command Reference

| Command | Shorthand / Flags | Description |
|---|---|---|
| `robo init` | `-y`, `--model <name>`, `--backend <gpu\|cpu>`, `--force` | Interactive setup wizard for models and runtime libraries. |
| `robo get` | `--model <name>`, `--litertlm-lib <version>`, `--no-ui` | Download model weights and native libraries directly to `~/.robo`. |
| `robo [prompt]` | `-y`, `-l`, `-c`, `-o <format>`, `--system <prompt>` | Synthesize and execute shell commands from natural language. |
| `robo daemon` | `start`, `stop`, `status` | Manage the background `robod` model server. |
| `robo version` | `-o json`, `-o plain` | Display version, commit SHA, and platform information. |

### Global Flags
* `-y`, `--auto-accept` — Automatically execute safe, non-destructive commands without prompting.
* `--yolo-approve-all` — Auto-accept and execute all commands, including destructive operations.
* `-l`, `--local` — Force execution on the local on-device SLM.
* `-c`, `--cloud` — Force execution on the cloud model.
* `-o`, `--output <format>` — Output format (`markdown`, `plain`, `json`, `code`).
* `--system <prompt>` — Custom system instruction override.

---

## Directory Structure

`robo` maintains all runtime state under `~/.robo`:

```text
~/.robo/
├── config.yaml          # Active configuration
├── robo.db              # SQLite execution history and contextual diagnostics
├── robod.json           # Daemon PID and loopback state
├── robod.log            # Background daemon logs and runtime diagnostics
├── cache/               # Local model weights (*.litertlm)
└── lib/                 # Native LiteRT-LM runtime binaries (v0.16.0/)
```

---

## License

Apache 2.0. See `LICENSE` for details.
