# Internal Release Checklist (Solo)

## Pre-Tag

- [ ] `go build ./cmd/focus` passes
- [ ] `go test ./...` reviewed (known env-limited failures documented)
- [ ] `./focus --help` works
- [ ] `./focus --version` prints the expected tag
- [ ] no repository file paths contain characters forbidden in Go module zips (e.g. full-width colon `：`)
- [ ] README reflects current product positioning
- [ ] `docs/adr/README.md` indexes all accepted/superseded ADRs
- [ ] `docs/architecture/` is consistent with current code (or marked as draft)
- [ ] `LICENSE` file present and correct
- [ ] release notes drafted (what changed, known risks, rollback target)

## Tag + CI

- [ ] tag created (`v0.x.y-internal.N` or `v0.x.y-rc.N`)
- [ ] tag pushed
- [ ] CI workflow triggered (`.github/workflows/internal-release.yml`)
- [ ] CI workflow has `permissions: contents: write` so it can publish the prerelease
- [ ] CI artifacts generated (linux/darwin/windows, amd64/arm64 where supported)
- [ ] `checksums.txt` generated
- [ ] GitHub prerelease published with artifacts attached

## Post-Release

- [ ] download/collect selected local platform binary
- [ ] install to `~/apps/focus/releases/<version>/focus`
- [ ] update stable symlink `~/.local/bin/focus`
- [ ] verify command path:
  - `which focus`
  - `focus --help`
  - `focus --version`
- [ ] keep previous version for rollback

## Release Notes Template

```markdown
## focus v0.x.y-internal.N

### What's changed
- ...

### Known risks / limitations
- ...

### Rollback target
- v0.x.y-internal.N-1
```
