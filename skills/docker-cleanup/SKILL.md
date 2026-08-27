---
name: docker-cleanup
description: Audits and reclaims host disk space from dangling images, stopped containers, build caches, and volumes.
triggers:
  keywords: ["docker prune", "docker cleanup", "dangling images", "docker disk", "reclaim docker space", "docker volumes", "docker system df"]
---

# Docker Storage Reclamation & Cleanup Guidelines

When auditing or freeing disk space consumed by Docker:

## 1. Pre-Flight Disk Space Audit
Always inspect disk allocation before running destructive pruning commands:
* Summary breakdown: `docker system df`
* Detailed verbose audit: `docker system df -v`

## 2. Tiered Cleanup Commands
Group cleanup operations by safety and risk level:

### Tier 1: Safe Non-Destructive Cleanup
* Remove dangling images (untagged layers): `docker image prune -f`
* Clean stopped containers: `docker container prune -f`
* Clear build cache: `docker builder prune -f`

### Tier 2: Comprehensive Cleanup
* Remove unused images (not just dangling): `docker image prune -a -f`
* System prune (containers, networks, dangling images): `docker system prune -f`

### Tier 3: Destructive Volume Cleanup (Requires Explicit Confirmation)
* Prune anonymous unused volumes: `docker volume prune -f`
* **Warning:** Never delete named volumes containing persistent database or state files without user confirmation.

## 3. Targeted Image Removal
* Remove specific image by ID/tag: `docker rmi <image_id>`
* Filter dangling images: `docker images -f "dangling=true" -q`
