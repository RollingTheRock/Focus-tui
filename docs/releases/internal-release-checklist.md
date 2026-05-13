# Internal Release Checklist (Solo)

## Pre-Tag

- [ ] `go build ./cmd/focus` passes
- [ ] `go test ./...` reviewed (known env-limited failures documented)
- [ ] README reflects current product positioning
- [ ] release notes drafted (what changed, known risks, rollback target)

## Tag + CI

- [ ] tag created (`v0.x.y-internal.N` or `v0.x.y-rc.N`)
- [ ] tag pushed
- [ ] CI artifacts generated (linux/darwin/windows, amd64/arm64 where supported)
- [ ] `checksums.txt` generated

## Post-Release

- [ ] download/collect selected local platform binary
- [ ] install to `~/apps/focus/releases/<version>/focus`
- [ ] update stable symlink `~/.local/bin/focus`
- [ ] verify command path:
  - `which focus`
  - `focus --help`
- [ ] keep previous version for rollback
