---
name: pr
description: Push current branch and create a pull request
disable-model-invocation: true
argument-hint: "[optional PR title]"
---

Create a pull request for the current branch. Follow these steps:

1. Run `git status` and `git log --oneline main..HEAD` (or master..HEAD) to understand all changes
2. Run `git diff main...HEAD --stat` to see the full scope
3. If not on a feature branch, create one from the current commits
4. Push to remote with `git push -u origin <branch>`
5. Create the PR with `gh pr create`:
   - Title: concise, under 70 chars (use $ARGUMENTS if provided)
   - Body format:
     ```
     ## Summary
     <bullet points of changes>

     ## Test plan
     <how to verify>

     🤖 Generated with [Claude Code](https://claude.com/claude-code)
     ```
6. Return the PR URL
