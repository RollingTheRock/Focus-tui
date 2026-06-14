# Clean Code Freeze & Migration Plan (Internal)

Date: 2026-05-13
Branch: feat-clean-code
Goal: Freeze or migrate non-primary modules without breaking current functionality.

## Non-Negotiable Guardrail

Do not change active runtime feature paths unless replacement is merged and verified in the same batch.
Reference baseline: `docs/research/2026-05-13-feature-implementation-matrix.md`.

## Module Decision Ledger

| Module/Area | Current Status | Decision | Action Type | Exit Criteria |
|---|---|---|---|---|
| `internal/a2a` | Not wired in runtime path | Freeze | Keep code, mark as frozen candidate | Runtime and tests unchanged; decision logged |
| `internal/sessiondigest` | Integration disabled in app layer | Freeze | Keep code, mark as frozen candidate | Runtime and tests unchanged; decision logged |
| `internal/worktree` | Package not wired, feature implemented elsewhere | Migrate to legacy track | Keep code for now; classify as legacy package | No imports introduced; no feature regressions |
| `internal/ui/pomodoro` | Panel package not registered, domain events remain | Migrate to legacy track | Keep package; postpone deletion until product decision | Product owner confirms panel fate |
| `.archive/*` | Legacy docs outside docs taxonomy | Migrate complete | Moved into `docs/archive` | No active docs references broken |
| `cmd/layouttest` | Dev-only helper command | Remove | Deleted | `go build ./cmd/focus` succeeds |
| `bin/focus` | Tracked local binary artifact | Remove from repo tracking path | Deleted | App entry still builds from source |

## Freeze Policy (for this branch)

A frozen module means:
1. No new feature development inside the module.
2. Only critical fix or compatibility change allowed.
3. README/docs must point to active implementation path first.

## Migration Policy

A migrated-to-legacy module means:
1. Module is retained for historical/rollback value.
2. Module is treated as non-authoritative path for new work.
3. Future removal requires a dedicated compatibility check batch.

## Completed in This Batch

1. Migrated legacy docs into `docs/archive/`.
2. Removed `cmd/layouttest`.
3. Removed tracked binary `bin/focus`.
4. Created feature-to-implementation safety matrix.

## Next Batch (No Deletion Yet)

1. Add status annotations (frozen/legacy) in package-level docs for:
   - `internal/a2a`
   - `internal/sessiondigest`
   - `internal/worktree`
   - `internal/ui/pomodoro`
2. Rewrite README to match current active implementation and internal-release stance.
3. Add `docs/releases/internal-release-process.md` and pre-release flow.

## Verification Commands

- `GOCACHE=/tmp/go-build go build ./cmd/focus`
- `GOCACHE=/tmp/go-build go test ./...`

Known environment caveat in sandbox: MCP HTTP/socket tests may fail due to permission restrictions.
