# Internal Release Process (Solo Maintainer)

Date: 2026-06-14  
Scope: `focus` internal / preview release workflow for single-maintainer operation.

## Goal

Create reliable internal builds that are easy to use daily, easy to roll back, and do not require heavyweight team process. At this stage all GitHub releases are **prereleases** (`v0.x.y-internal.N` or `v0.x.y-rc.N`); no stable public semver is claimed.

## Versioning

Use pre-release style versions only:

- `v0.x.y-internal.N` for frequent internal builds
- `v0.x.y-rc.N` for broader internal validation

Do not publish stable public-semver claims in this stage.

Examples:

```text
v0.2.0-internal.1
v0.2.0-rc.1
v0.2.1-internal.3
```

## Release Sources

- **Trigger**: git tag push matching:
  - `v*-internal.*`
  - `v*-rc.*`
- **Branch**: any branch can host the tag, but the following are recommended:
  - `release/internal` for internal builds (create if it does not exist)
  - `release/v0.x` for release candidates
  - `master` / `main` for hotfix tags

The CI workflow (`.github/workflows/internal-release.yml`) is tag-triggered; it does not enforce a specific branch.

## What the CI workflow does

1. Builds `focus` for:
   - linux/amd64, linux/arm64
   - darwin/amd64, darwin/arm64
   - windows/amd64
2. Uploads per-platform artifacts.
3. Generates `checksums.txt`.
4. Publishes a GitHub **prerelease** attached to the tag.

The GitHub prerelease is public, but the version string (`-internal.N` / `-rc.N`) and the prerelease flag make it clear this is not a stable release.

## CI Permissions

The release workflow must declare `permissions: contents: write` at the workflow level (or on the release job). The default `GITHUB_TOKEN` only has read access to repository contents; without this declaration the `Publish GitHub prerelease` step fails with:

```text
Resource not accessible by integration
```

See `.github/workflows/internal-release.yml` for the current declaration.

## Minimal Release Steps

1. Sync the branch you want to release from.
2. Run local gates:
   - `go build ./cmd/focus`
   - `go test ./...` (non-restricted environment recommended)
3. Run smoke checks:
   - `./focus --help`
   - `./focus --version`
4. Tag the release:
   ```bash
   git tag v0.2.0-internal.1
   git push origin v0.2.0-internal.1
   ```
5. Wait for CI to finish and confirm artifacts appear in the GitHub prerelease.
6. Download the binary for your platform and bind the local `focus` command to this version.

## Local Stable Binding (Daily Use)

Recommended layout:

- `~/apps/focus/releases/<version>/focus` (stored binary)
- `~/.local/bin/focus` (stable symlink)

Example:

```bash
VERSION="v0.2.0-internal.1"
mkdir -p ~/apps/focus/releases/${VERSION}
cp ./focus ~/apps/focus/releases/${VERSION}/focus
ln -sfn ~/apps/focus/releases/${VERSION}/focus ~/.local/bin/focus
focus --help
```

## Rollback

Rollback is symlink switch only:

```bash
ln -sfn ~/apps/focus/releases/<previous-version>/focus ~/.local/bin/focus
focus --help
```

## Notes

- Keep at least the last 2 internal versions locally for rollback.
- If a release fails smoke checks, publish `internal.N+1` after the fix instead of mutating old tags.
- When the project is ready for a public stable release, retire the `-internal` suffix and use normal semver with `prerelease: false`.
