---
name: git-commit
description: Analyzes git diff and status to generate Conventional Commit messages.
triggers:
  keywords: ["commit", "git commit", "staged", "commit message", "write commit"]
  files: [".git/*", ".gitignore"]
---

# Git Conventional Commit Guidelines

When assisting the user with generating git commit messages or commands:
1. Format: `<type>(<scope>): <short summary>` (50 characters or less).
   - Valid types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.
2. Commit Body: Explain the motivation, rationale, and 'why' for the change rather than merely restating the code diff.
3. Pre-Flight Verification: Always inspect working tree state (`git status --porcelain` or `jj status`) before proposing commit operations.
4. Safeguards: Never generate commands that execute `git push --force` to `main`, `master`, or production branches.
