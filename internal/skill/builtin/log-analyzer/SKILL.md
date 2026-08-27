---
name: log-analyzer
description: Queries, filters, and extracts error traces from system journals, kernel rings, and log files.
version: 1.0.0
triggers:
  keywords: ["journalctl", "dmesg", "syslog", "check logs", "service logs", "system errors", "crash log", "event viewer", "kernel errors", "tail logs"]
---

# Local Log Analysis & Diagnostics Guidelines

When querying system journals, kernel diagnostics, or service error logs:

## 1. Systemd Journalctl Queries (Linux)
* **Specific Service Unit Logs:**
  * Recent entries with explanations: `journalctl -u <service_name> -n 50 --no-pager`
  * Filter by priority (errors & warnings): `journalctl -u <service_name> -p err..warning -n 50 --no-pager`
  * Time-bounded logs: `journalctl -u <service_name> --since "1 hour ago" --no-pager`
* **System Boot & Kernel:**
  * Current boot errors: `journalctl -b -p err --no-pager`
  * Previous boot crash logs: `journalctl -b -1 -p err --no-pager`

## 2. Kernel Ring Buffer (dmesg)
* Human-readable timestamps with error level filtering:
  `dmesg -T --level=err,warn | tail -n 30`
* Search for hardware / OOM / driver events:
  `dmesg -T | grep -iE "oom|out of memory|segfault|error|killed"`

## 3. Flat Log Files (/var/log)
* Inspect recent lines: `tail -n 50 /var/log/syslog` (or `/var/log/messages`)
* Follow log in real-time (when requested): `tail -f /var/log/<file>`

## 4. Windows Event Logs (PowerShell)
* Query recent Application errors:
  `Get-WinEvent -FilterHashtable @{LogName='Application'; Level=2; StartTime=(Get-Date).AddHours(-1)} -MaxEvents 20 | Select-Object TimeCreated, Id, Message`
* Query recent System errors:
  `Get-WinEvent -FilterHashtable @{LogName='System'; Level=2; StartTime=(Get-Date).AddHours(-1)} -MaxEvents 20 | Select-Object TimeCreated, Id, Message`

## 5. Non-Interactive Rule
Always include `--no-pager` for `journalctl` and pipe large streams to `head` / `tail` to prevent terminal paging lockups.
