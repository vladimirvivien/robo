---
name: network-triage
description: Diagnoses listening sockets, port conflicts, DNS resolution, and local connectivity.
version: 1.0.0
triggers:
  keywords: ["port conflict", "listening ports", "port in use", "dns lookup", "check connection", "netstat", "ss -tlpn", "curl error", "socket", "open port"]
---

# Network Triage & Socket Diagnostics Guidelines

When diagnosing local networking issues, port conflicts, or socket bindings:

## 1. Listening Ports & Owning Process Mapping
* **Linux / macOS:**
  * Fast listening socket audit: `ss -tlpn` (Linux) or `lsof -iTCP -sTCP:LISTEN -P -n` (Linux/macOS).
  * Specific port check: `lsof -i :<port>` or `ss -tlpn | grep :<port>`.
* **Windows PowerShell:**
  * Listening TCP ports: `Get-NetTCPConnection -State Listen | Select-Object -Property LocalPort, OwningProcess | Sort-Object LocalPort`.
  * Map port to process name: `Get-Process -Id (Get-NetTCPConnection -LocalPort <port> -ErrorAction SilentlyContinue).OwningProcess`.

## 2. Local DNS & Socket Reachability
* **DNS Resolution:**
  * Test lookup: `nslookup <hostname>` or `dig +short <hostname>`.
  * PowerShell: `Resolve-DnsName -Name <hostname>`.
* **TCP Socket Probing:**
  * Test port reachability: `nc -zv <host> <port>` or `curl -Iv http://<host>:<port>`.
  * PowerShell: `Test-NetConnection -ComputerName <host> -Port <port>`.

## 3. Resolving Port Conflicts (e.g. "Address already in use")
1. Query the owning PID occupying the target port.
2. Display the process name and command line before suggesting actions.
3. Suggest either:
   - Terminating the old process blocking the port.
   - Configuring the new service to bind to an alternate port.
