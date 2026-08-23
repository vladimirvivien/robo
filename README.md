# 🤖 robo

An small AI assistant for system operation in the terminal.

`robo` runs a small language model (Gemma 4 by default) locally to generate platform-specific shell commands, analyze terminal diagnostics, and execute multi-step developer workflows directly in your active shell.

---

## Key Capabilities

* **On-Device Local Inference:** Runs quantized models locally via LiteRT-LM (i.e. Gemma models) for private, offline execution.
* **Autonomous Multi-Step Execution:** Plans and executes multi-turn tasks across shell commands, inspecting each step's output to determine subsequent actions.
* **Dynamic Platform Dialect Prompting:** Automatically detects your OS and active shell (PowerShell, Bash, Zsh, Fish, CMD) and constructs compact system prompts tailored specifically to your target environment.
* **Ambient Shell History Awareness:** Ingests recent terminal command history to contextualize requests without manual copy-pasting.
* **Background Daemon (`robod`):** Keeps local models resident in memory for fast execution, automatically starting on demand and shutting down after inactivity.
* **Interactive Safety & Guardrails:** Evaluates command risk (Read-Only, Mutating, Destructive) with an interactive review menu (`[Run]`, `[Edit]`, `[Cancel]`) and typed confirmation gates for destructive operations (`--yes-allow-destructive`).

---

## Installation & Build

### Prerequisites
* Go 1.24+ and git

### Build from Source
```bash
# Clone the repository
git clone https://github.com/vladimirvivien/robo.git
cd robo

make build

# Or, build and add to $PATH
go install .

# Verify installation
robo version
```

---

## Quickstart

### 1. Initialize and Download Local Model
Initialize `robo` to select your local model and hardware backend:

```bash
# Initialize robo
robo init
```

### 2. Inspect System & Model Status
Verify your active configuration, local SLM runtime, and daemon state:

```bash
robo status
```

### 3. Run Your First Query
Ask `robo` to perform a task in natural language:

```bash
robo "what is the process using the most CPU"
```

`robo` synthesizes the exact platform-specific command and presents an interactive review menu:
```text
# Windows:

╭──────────────────────────────────────────────────────────────────────────────╮
│ 🤖 Proposed Shell Command                                                    │
│                                                                              │
│    Get-Process | Sort-Object CPU -Descending | Select-Object -First 5        │
╰──────────────────────────────────────────────────────────────────────────────╯

# Linux/Unix:

╭──────────────────────────────────────────────────────────────────────────────╮
│ 🤖 Proposed Shell Command                                                    │
│                                                                              │
│    ps aux --sort=-%cpu | head -n 6                                           │
╰──────────────────────────────────────────────────────────────────────────────╯

┃ Execute command?
┃ > Run command
┃   Edit command
┃   Cancel
```

---

## Real-World Workflows & Examples

The following lists possible ways of using `robo` with your local system. Due to the nature of language model generation, the output will not be exact as shown below.

### 1. Windows • Port Conflict Triage
Investigate a blocked network port, identify the offending process, stop it, and verify that the socket is cleared:

```powershell
robo "Check if port 8080 is in use. If so, identify the process name and PID, terminate it, and verify the port is released." --max-steps 4
```
* **Step 1 (Inspect)**: Runs `Get-NetTCPConnection -LocalPort 8080` to retrieve the owning PID.
* **Step 2 (Evaluate)**: Queries process metadata (`Get-Process -Id <PID>`) to verify process name and memory usage.
* **Step 3 (Remediate)**: Executes `Stop-Process -Id <PID>` with Tier 2 safety classification.
* **Step 4 (Verify)**: Re-checks port 8080 to confirm socket release and returns the final status.

---

### 2. Linux • Service Log Triage via Unix Pipe (`stdin`)
Pipe service logs, compiler errors, or kernel diagnostics directly into `robo` for root-cause analysis:

```bash
journalctl -u docker.service -n 50 --no-pager | robo "Identify why the daemon failed to start, extract error codes, and provide the fix."
```
* **Pipeline Ingestion**: `robo` ingests standard input into prompt context without manual copy-pasting.
* **Diagnostic Synthesis**: Extracts specific error codes and outputs actionable fix instructions.

---

### 3. Cross-Platform • Autonomous Build Failure & Self-Correction
Run a project build, capture compiler errors or missing modules, apply the remediation step, and re-verify the build:

```bash
robo "Run 'go build ./...'. If it fails due to missing modules or outdated dependencies, run 'go mod tidy' and verify if the build succeeds." --max-steps 3 -y
```
* **Step 1 (Execute)**: Runs `go build ./...` and captures compiler output and non-zero exit code.
* **Step 2 (Self-Correct)**: Detects missing dependencies and runs `go mod tidy`.
* **Step 3 (Re-Verify)**: Re-runs `go build ./...`, captures the clean `0` exit code, and reports success.

---

### 4. Linux • Structured JSON Output for Automation & `jq`
Generate strictly structured JSON from natural language to feed into downstream shell scripts:

```bash
robo "Inspect git status and list all untracked and modified files categorized by top-level directory" -o json | jq '.categories[] | select(.directory == "internal")'
```
* **Deterministic Formatting**: `-o json` enforces structured JSON output without conversational markdown wrappers.
* **CLI Interoperability**: Seamlessly pipes into standard Unix utilities (`jq`, `cut`, `awk`, `xargs`) for programmatic workflows.

---

### 5. Windows • System Resource & Process Sampling
Inspect hardware resource consumers, sample performance metrics over a time interval, and format the diagnostic report:

```powershell
robo "Find the top 3 processes consuming the most memory, sample their CPU usage over 3 seconds, and report the results." --max-steps 3
```
* **Step 1 (Identify)**: Runs `Get-Process | Sort-Object WS -Descending | Select-Object -First 3` to find top memory consumers.
* **Step 2 (Sample)**: Uses `Start-Sleep -Seconds 3` and queries CPU usage deltas for those specific PIDs.
* **Step 3 (Report)**: Synthesizes a compact markdown table comparing memory footprint and CPU utilization.

---

### 6. Linux • Autonomous Stale Artifact Discovery & Safe Cleanup
Locate stale caches or temporary build artifacts, calculate total reclaimable disk space, and execute cleanup:

```bash
robo "Find all .log and core dump files older than 7 days in build directories, calculate total space, and clean them up." --max-steps 3
```
* **Step 1 (Discover)**: Runs `find . -type f \( -name "*.log" -o -name "core.*" \) -mtime +7` to locate matching files.
* **Step 2 (Aggregate)**: Calculates cumulative file size and displays the breakdown.
* **Step 3 (Safety Gate)**: Proposes the deletion command (`rm -f`), evaluating the action against Tier 2 safety rules and prompting for confirmation.

---

## CLI Command Reference

| Command | Flags | Description |
|---|---|---|
| `robo [intent]` | `-y`, `-d`, `-1`, `--max-steps <N>`, `-o <format>`, `--system <prompt>` | Translate natural language into commands and execute multi-step tasks. |
| `robo init` | `-y`, `--model <name>`, `--backend <gpu\|cpu>`, `--version <ver>`, `--force` | Setup wizard for models, runtime libraries, and configuration. |
| `robo status` | `--json`, `--config <path>` | Display resolved configuration, SLM runtime, LLM status, and daemon process state. |
| `robo get` | `--model <name>`, `--litertlm-lib <ver>`, `--no-ui` | Download model weights and native libraries directly to `~/.robo`. |
| `robo history` | `--limit <N>`, `--clear`, `--json` | View or clear recorded command executions from the SQLite store. |
| `robo daemon` | `start`, `stop`, `status` | Manage the background `robod` model server process. |
| `robo version` | `-o json`, `-o plain` | Display binary version, commit SHA, build date, and platform. |

### Global Flags
* `-y`, `--yolo` — Auto-accept safe (Read-Only and Mutating) commands without prompting. Destructive actions still require explicit confirmation.
* `-d`, `--dry-run` — Simulate execution plan and tool proposals without executing commands against the host OS.
* `-1`, `--one-shot` — Force strictly single-turn execution ($N=1$) without follow-up autonomous loops.
* `--max-steps <N>` — Maximum number of agent loop completion steps (default: `5`).
* `-o`, `--output <format>` — Output format (`markdown`, `plain`, `json`, `code`).
* `--config <path>` — Custom configuration file path (default: `~/.robo/config.yaml`).
* `--system <prompt>` — Custom system instruction override.

---

## Configuration (`~/.robo/config.yaml`)

```yaml
robo:
  inference_mode: slm       # "slm" (on-device local) | "llm" (cloud frontier) | "auto"
  output_mode: markdown     # "markdown" | "plain" | "json" | "code"
  capture_history: true     # ingest recent shell history for context
  max_history_lines: 10     # maximum history lines to inspect
  auto_accept: false        # default yolo mode
  yolo_approve_all: false   # bypass destructive confirmation gates

slm:
  model: litert-community/gemma-4-E4B-it
  backend: gpu              # "gpu" | "cpu"
  max_tokens: 4096
  cache_dir: ~/.robo/cache
  version: v0.16.0
  auto_download: true

llm:                        # Optional: only needed if inference_mode: llm
  provider: googleai        # "googleai" | "anthropic" | "openai"
  model: googleai/gemini-2.5-flash
  api_key_env: GEMINI_API_KEY

robod:
  enabled: true             # enable background hot-start daemon
  idle_ttl: 15m0s           # shutdown after inactivity
  url: http://127.0.0.1:8765
```

---
## Known issue(s)

### Limited Small Model Knowledge

Using a small models like Gemma-4 2B or 4B (or even smaller) means `robo` performs better on well-known commands and tools. Smaller language models lack the general knowledge of less popular tools or commands and may require additional steering or prompt restructure to get results.

### Subshell Isolation Note

Commands executed by `robo` run in an isolated child subshell (`powershell -Command` on Windows, `bash -c` on Linux/macOS). 

Session-specific mutations—such as environment variables (`export`, `$env:`), virtual environments (`source`, `conda activate`), and other shell-bound changes, changes the subshell process and do not mutate the parent terminal. When a task requires persistent session state, `robo` explains this boundary and provides the command for direct execution.

---

## License

Apache 2.0. See `LICENSE` for details.
