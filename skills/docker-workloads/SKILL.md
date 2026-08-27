---
name: docker-workloads
description: Manages local container lifecycles, health checks, networks, and streaming logs.
triggers:
  keywords: ["docker container", "docker run", "docker compose", "container logs", "docker inspect", "container status", "restart container", "docker ps"]
  files: ["Dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"]
---

# Docker Workload & Container Operations Guidelines

When managing local Docker containers, Compose projects, or debugging runtime issues:

## 1. Container Status & Health Inspection
* List running and stopped containers: `docker ps -a --format "table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}"`.
* Inspect detailed container state (IP, mounts, env): `docker inspect <container_id_or_name>`.
* Extract exit code and error state of failed container:
  `docker inspect <container> --format '{{.State.ExitCode}} - {{.State.Error}}'`.

## 2. Container Log Diagnostics
* Tail recent logs with timestamps: `docker logs --tail 50 -t <container>`.
* Compose services: `docker compose logs --tail 50 <service_name>`.
* Inspect unbuffered stdout/stderr when container crashes immediately upon startup.

## 3. Network & Port Mapping
* List container networks: `docker network ls`.
* Inspect bridge network attachments and IP assignments: `docker network inspect bridge`.
* Verify published port mappings: `docker port <container>`.

## 4. Operational Best Practices
* For background services, always use detached mode (`-d`).
* Explicitly name containers (`--name <name>`) and set restart policies (`--restart unless-stopped`) when appropriate.
* Never execute interactive shell commands (`docker exec -it`) without headless alternatives.
