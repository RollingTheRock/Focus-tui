---
name: commit
description: Stage changes, generate a commit message, and commit
disable-model-invocation: true
argument-hint: "[optional message override]"
---

Commit the current changes. Follow these steps:

1. Run `git status` and `git diff --stat` to understand what changed
2. Run `git log --oneline -5` to see the project's commit message style
3. Stage relevant files (avoid binaries, .env, credentials). Prefer `git add <specific files>` over `git add -A`
4. Generate a concise commit message:
   - First line: imperative mood, under 72 chars, describes the "what"
   - Blank line, then body if needed: describes the "why"
   - End with: `Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>`
5. If the user provided an override message via $ARGUMENTS, use that instead
6. Create the commit
7. Show the result with `git log --oneline -1`
