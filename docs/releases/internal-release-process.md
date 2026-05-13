# Internal Release Process (Solo Maintainer)

Date: 2026-05-13
Scope: `focus` internal-only release workflow for single maintainer operation.

## Goal

Create reliable internal builds that are easy to use daily, easy to roll back, and do not require heavyweight team process.

## Versioning

Use pre-release style versions only:

- `v0.x.y-internal.N` for frequent internal builds
- `v0.x.y-rc.N` for broader internal validation

Do not publish stable public-semver claims in this stage.

## Release Sources

- Branch: `release/internal` (recommended)
- Trigger: git tag push matching:
  - `v*-internal.*`
  - `v*-rc.*`

## Minimal Release Steps

1. Sync release branch
2. Run local gates:
   - `go build ./cmd/focus`
   - `go test ./...` (non-restricted environment recommended)
3. Tag release:
   - `git tag v0.1.0-internal.1`
   - `git push origin v0.1.0-internal.1`
4. GitHub Actions builds multi-platform artifacts and checksums
5. Download artifacts and bind local `focus` command to this stable internal version

## Local Stable Binding (Daily Use)

Recommended layout:

- `~/apps/focus/releases/<version>/focus` (stored binary)
- `~/.local/bin/focus` (stable symlink)

Example:

```bash
mkdir -p ~/apps/focus/releases/v0.1.0-internal.1
cp ./focus ~/apps/focus/releases/v0.1.0-internal.1/focus
ln -sfn ~/apps/focus/releases/v0.1.0-internal.1/focus ~/.local/bin/focus
focus --help
```

## Rollback

Rollback is symlink switch only:

```bash
ln -sfn ~/apps/focus/releases/<previous-version>/focus ~/.local/bin/focus
focus --help
```

## Notes

- Keep at least last 2 internal versions locally for rollback.
- If a release fails smoke checks, publish `internal.N+1` after fix instead of mutating old tags.
