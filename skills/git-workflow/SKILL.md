---
name: git-workflow
description: Manages git branch operations, stash lifecycles, conflict resolution, and working tree triage.
triggers:
  keywords: ["git status", "git stash", "merge conflict", "git branch", "git clean", "untracked files", "discard changes", "git diff", "git log", "git rebase"]
  files: [".git/*", ".gitignore"]
---

# Git Workflow & Working Tree Triage Guidelines

When assisting with Git version control, merge conflicts, or working tree state:

## 1. Non-Destructive State Inspection
* Check status concisely: `git status -s` or `git status`
* View uncommitted diff: `git diff` (working tree) or `git diff --cached` (staged)
* Compact branch history: `git log --oneline -n 10 --graph --decorate`

## 2. Merge & Rebase Conflict Resolution
* Detect conflict markers: `git diff --check`
* List unmerged conflicted files: `git status --porcelain | grep "^UU\|^AA\|^DU\|^UD"`
* Workflow rules during conflict:
  1. Guide user to edit conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`).
  2. Mark resolved: `git add <file>`.
  3. Continue: `git rebase --continue` or `git merge --continue`.
  4. Abort safety: Provide `git rebase --abort` or `git merge --abort` if user requests to cancel.

## 3. Stash Management
* Save uncommitted work: `git stash push -m "WIP: <description>"`
* List saved stashes: `git stash list`
* Inspect top stash: `git stash show -p stash@{0}`
* Apply & pop: `git stash pop`

## 4. Working Tree Cleanup Safeguards
* **Dry run first:** Always run `git clean -n -d` before deleting untracked files.
* Never generate `git clean -fdx` or `git reset --hard` without explaining that untracked or uncommitted work will be permanently lost.
* Never generate `git push --force` targeting `main`, `master`, or release branches.
