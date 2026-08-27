---
name: security-permissions
description: Audits local file permissions, SSH key directories, world-writable files, and sudo privileges.
triggers:
  keywords: ["chmod", "chown", "ssh key permissions", "permission denied", "sudoers", "world-writable", "file permissions", "ssh-keygen", "fix permissions"]
---

# Security, Permissions & SSH Key Audit Guidelines

When auditing or correcting local file permissions, SSH key security, or diagnosing permission errors:

## 1. SSH Key Directory Hardening
SSH clients enforce strict permissions. If permissions are too open, SSH authentication fails:
* Correct `~/.ssh` directory permissions: `chmod 700 ~/.ssh`
* Correct private key permissions: `chmod 600 ~/.ssh/id_* ~/.ssh/*.pem`
* Correct public key permissions: `chmod 644 ~/.ssh/id_*.pub ~/.ssh/authorized_keys ~/.ssh/known_hosts`
* Windows PowerShell SSH ACLs:
  `icacls $env:USERPROFILE\.ssh\id_rsa /inheritance:r /grant:r "$($env:USERNAME):(R,W)"`

## 2. World-Writable & SUID File Auditing
* Find world-writable files in current workspace or system:
  `find . -type f -perm -002 -ls 2>/dev/null`
* Find unexpected SUID binaries on Linux:
  `find / -perm -4000 -type f -exec ls -la {} + 2>/dev/null`

## 3. Ownership & Group Corrections
* Change user/group ownership: `sudo chown -R <user>:<group> <path>`
* Recursive directory permissions standard (directories 755, files 644):
  * `find <path> -type d -exec chmod 755 {} +`
  * `find <path> -type f -exec chmod 644 {} +`

## 4. Sudo & Permission Denied Diagnostics
* Check current user sudo privileges: `sudo -l`
* When diagnosing "Permission denied", check:
  1. File ownership (`ls -ld <path>`).
  2. Read/Write/Execute bits for owner, group, other.
  3. Parent directory execute (`+x`) permissions needed to traverse paths.
