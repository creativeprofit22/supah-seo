---
name: commit
description: Run checks, commit with AI message, and push
---

1. Run quality checks in order. Fix ALL errors before continuing:
   ```bash
   make fmt
   make vet
   make test
   make lint
   ```

2. Review changes: `git status` and `git diff`

3. Generate commit message per project policy:
   - Subject: one line starting with `Add`, `Update`, `Fix`, `Remove`, or `Refactor`
   - Body: short bullets grouped under `Added:` / `Updated:` / `Fixed:` / `Docs:` (omit empty sections)
   - Concrete, file/function/behavior specific

4. Commit and push:
   ```bash
   git add -A
   git commit -m "your generated message"
   git push
   ```
