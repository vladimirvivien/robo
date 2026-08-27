---
name: service-management
description: Controls, restarts, enables/disables, and inspects status of local background services.
triggers:
  keywords: ["systemctl", "restart service", "service status", "enable service", "daemon status", "start service", "launchctl", "windows service", "failed services"]
---

# Service Management & Daemon Lifecycle Guidelines

When managing background services, system daemons, or investigating failed service states:

## 1. Systemd (Linux)
* **Check Service Status:** `systemctl status <service_name>.service`
* **List Failed Units:** `systemctl --failed`
* **Lifecycle Actions:**
  * Start: `sudo systemctl start <service_name>`
  * Stop: `sudo systemctl stop <service_name>`
  * Restart: `sudo systemctl restart <service_name>`
  * Reload config (graceful): `sudo systemctl reload <service_name>`
* **Enable / Disable at Boot:**
  * Enable: `sudo systemctl enable --now <service_name>`
  * Disable: `sudo systemctl disable <service_name>`
* **Reload Daemon Definitions:** `sudo systemctl daemon-reload` (after editing unit files).

## 2. macOS (launchd)
* List loaded agents/daemons: `launchctl list | grep <pattern>`
* Load & Start: `launchctl load -w ~/Library/LaunchAgents/<file>.plist`
* Unload & Stop: `launchctl unload -w ~/Library/LaunchAgents/<file>.plist`
* Bootout / Bootstrap (macOS 10.11+): `launchctl bootstrap gui/$UID ~/Library/LaunchAgents/<file>.plist`

## 3. Windows PowerShell Services
* Check status: `Get-Service -Name <service_name>`
* List running/stopped: `Get-Service | Where-Object {$_.Status -eq "Running"}`
* Start/Stop/Restart:
  * `Start-Service -Name <service_name>`
  * `Stop-Service -Name <service_name>`
  * `Restart-Service -Name <service_name>`
* Set Startup Type: `Set-Service -Name <service_name> -StartupType Automatic`

## 4. Diagnostic Rules
* When a service fails to start, immediately follow up by querying recent error logs (`journalctl -u <service_name> -xe` on Linux).
