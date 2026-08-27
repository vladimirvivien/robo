---
name: sys-diagnostics
description: Inspects system resource usage, open ports, and process diagnostics.
version: 1.0.0
triggers:
  keywords: ["memory", "cpu", "disk", "ports", "listening ports", "top processes", "resource usage", "process table"]
---

# System Diagnostics Operating Guidelines

When diagnosing system health, performance, or querying processes and network ports:
1. Processes & Resource Ranking:
   - Linux: Use `ps aux --sort=-%cpu | head -n 6` or `ps aux --sort=-%mem | head -n 6`. For memory overview use `free -h`.
   - macOS: Use `ps aux -r | head -n 6` (CPU) or `top -l 1 | grep PhysMem` (Memory).
   - Windows PowerShell: Use `Get-Process | Sort-Object CPU -Descending | Select-Object -First 5` or `Get-Process | Sort-Object WS -Descending | Select-Object -First 5`.
2. Listening Ports:
   - Linux / macOS: Use `ss -tlpn` or `lsof -iTCP -sTCP:LISTEN`.
   - Windows PowerShell: Use `Get-NetTCPConnection -State Listen | Select-Object -Property LocalPort, OwningProcess`.
3. Disk Usage:
   - Linux / macOS: Use `df -h`.
   - Windows PowerShell: Use `Get-PSDrive -PSProvider FileSystem | Select-Object Name, Used, Free`.
4. Command Style: Always generate non-interactive batch commands. Avoid launching interactive pagers (`top`, `htop`, `less`).
