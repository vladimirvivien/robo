---
name: tool-installer
description: Installs, updates, and verifies developer CLI tools across native OS package managers.
version: 1.0.0
triggers:
  keywords: ["install tool", "install cli", "brew install", "apt install", "winget", "update tool", "package manager", "missing command", "install package"]
---

# Developer Tool & Package Installation Guidelines

When assisting with installing, updating, or resolving missing CLI developer tools:

## 1. Package Manager Detection & Commands
Detect the host operating system and generate idiomatic package manager commands:

* **macOS (Homebrew):**
  * Install: `brew install <package>` (or `brew install --cask <app>`)
  * Update: `brew upgrade <package>`
  * Search: `brew search <query>`
* **Ubuntu / Debian (APT):**
  * Install: `sudo apt-get update && sudo apt-get install -y <package>`
* **RHEL / Fedora (DNF):**
  * Install: `sudo dnf install -y <package>`
* **Arch Linux (Pacman):**
  * Install: `sudo pacman -S --noconfirm <package>`
* **Windows (Winget / Scoop / Chocolatey):**
  * Winget (Default): `winget install --id <Package.ID> -e --silent --accept-source-agreements --accept-package-agreements`
  * Scoop: `scoop install <package>`
  * Chocolatey: `choco install <package> -y`

## 2. Post-Install Verification
Always verify that the installed binary is accessible in the active shell environment:
* POSIX: `which <binary_name> && <binary_name> --version`
* Windows PowerShell: `Get-Command <binary_name> -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source`

## 3. PATH Troubleshooting
If a tool is installed but not found in the current shell:
* Advise restarting the terminal or sourcing the shell profile (`source ~/.zshrc` / `source ~/.bashrc`).
* On Windows, explain that new `PATH` entries take effect in newly launched shell sessions.
