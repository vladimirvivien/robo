---
name: disk-storage-triage
description: Identifies full filesystems, scans large directories/files, and checks inode utilization.
version: 1.0.0
triggers:
  keywords: ["disk full", "disk space", "largest files", "df -h", "du -sh", "out of space", "clean disk", "free space", "disk usage"]
---

# Disk Storage & Large File Triage Guidelines

When analyzing disk space exhaustion, directory footprints, or filesystem health:

## 1. Filesystem Allocation & Inode Audit
* **Linux / macOS:**
  * Filesystem free space: `df -h`
  * Inode utilization (check for inode exhaustion): `df -i`
* **Windows PowerShell:**
  * Volume free space: `Get-PSDrive -PSProvider FileSystem | Select-Object Name, @{Name="Used(GB)";Expression={[math]::Round($_.Used/1GB,2)}}, @{Name="Free(GB)";Expression={[math]::Round($_.Free/1GB,2)}}`

## 2. Directory Footprint Ranking
* **Top 10 space consumers in current directory:**
  * Linux / macOS: `du -sh ./* 2>/dev/null | sort -hr | head -n 10`
* **Top consumers across a specific path (e.g. `/var/log`):**
  * `du -ah /var/log 2>/dev/null | sort -rh | head -n 10`
* **Windows PowerShell:**
  * `Get-ChildItem -Directory | ForEach-Object { $size = (Get-ChildItem $_.FullName -Recurse -File -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum; [PSCustomObject]@{ Directory=$_.Name; SizeMB=[math]::Round($size/1MB,2) } } | Sort-Object SizeMB -Descending | Select-Object -First 10`

## 3. Large File Discovery
* Find files larger than 500MB:
  * Linux / macOS: `find . -type f -size +500M -exec ls -lh {} + 2>/dev/null | awk '{ print $5, $9 }'`
  * Windows PowerShell: `Get-ChildItem -Recurse -File | Where-Object { $_.Length -gt 500MB } | Select-Object FullName, @{Name="Size(MB)";Expression={[math]::Round($_.Length/1MB,2)}}`

## 4. Common Cleanup Candidates
* Package manager caches: `apt clean`, `brew cleanup`, `dnf clean all`
* Old system logs: `journalctl --vacuum-size=500M` or `journalctl --vacuum-time=7d`
* Temp folders: `/tmp`, `~/.cache`
