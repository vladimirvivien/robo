---
name: process-management
description: Inspects process trees, resource consumers, finds PIDs, and manages process termination.
triggers:
  keywords: ["kill process", "stop process", "find pid", "high cpu process", "process tree", "zombie process", "top processes", "ps aux", "terminate process"]
---

# Process Management & Resource Triage Guidelines

When assisting with process inspection, resource consumption, or process termination:

## 1. Process Discovery & PID Lookup
* **Linux / macOS:**
  * Find PID by name pattern: `pgrep -fa <pattern>` (or `ps aux | grep -i <pattern> | grep -v grep`).
  * Process tree hierarchy: `pstree -p <PID>` or `ps -ef --forest`.
* **Windows PowerShell:**
  * Find process: `Get-Process | Where-Object { $_.ProcessName -like "*<pattern>*" } | Select-Object Id, ProcessName, CPU, WS`.
  * For exact PID lookup: `Get-Process -Id <PID>`.

## 2. Resource & Consumption Ranking
* **Linux:** Top CPU consumers: `ps aux --sort=-%cpu | head -n 6` (or `sort=-%mem` for memory).
* **macOS:** Top CPU consumers: `ps aux -r | head -n 6`. Top memory: `top -l 1 -o mem | head -n 15`.
* **Windows PowerShell:**
  * Top CPU: `Get-Process | Sort-Object CPU -Descending | Select-Object -First 5 Id, ProcessName, CPU, WS`.
  * Top Memory: `Get-Process | Sort-Object WS -Descending | Select-Object -First 5 Id, ProcessName, @{Name="MB";Expression={[math]::Round($_.WS/1MB,2)}}`.

## 3. Safe Termination Protocol
1. **Always verify the exact PID** and executable name before issuing kill signals.
2. **Graceful Termination First:**
   * Linux / macOS: Send `SIGTERM` first (`kill <PID>` or `pkill -15 <pattern>`).
   * Windows: `Stop-Process -Id <PID>`.
3. **Forceful Termination (Fallback Only):**
   * Use `kill -9 <PID>` or `Stop-Process -Id <PID> -Force` only after graceful termination fails or when explicitly requested.
4. **Safety Constraint:** Never terminate PID 1, system init daemons, or security agents.
