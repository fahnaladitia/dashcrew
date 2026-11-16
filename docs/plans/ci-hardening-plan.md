# CI/CD hardening audit

Tier: standard. `.github/workflows/` (ci, release), `.github/dependabot.yml`, AGENTS.md (Verify), plus repository settings only the maintainer can change. No Go code, no generated output.

## Already in place (keep)

| Area | State |
|---|---|
| Action pinning | Every action pinned to a full commit SHA with the version in a comment |
| Checkout | `persist-credentials: false` in every job |
| Token scope | Top-level `contents: read` (ci) or `{}` (codeql, release); write only where needed (`security-events`, release `contents`) |
| Repo token default | Workflow token `read`; Actions cannot approve pull requests |
| Triggers | No `pull_request_target`, no `workflow_run`; release only on `v*` tags |
| Checks | Test on three OSes, `-race`, `gofmt`, `govulncheck@v1.9.0`, CodeQL on PR, push and weekly |
| Required on `main` | test x3, gofmt, race, govulncheck, analyze (ruleset "main protection") |
| Dependencies | Dependabot for gomod and github-actions, weekly, 14-day cooldown |
| Secrets | Secret scanning and push protection on; GitGuardian on PRs; one secret (`HOMEBREW_TAP_TOKEN`) |
| Disclosure | Private vulnerability reporting on; SECURITY.md |
| Build | `CGO_ENABLED=0`, `-trimpath`, `go mod tidy -diff` before build, `checksums.txt` |

## Gaps, by priority

### High

1. **`govulncheck` runs only on a push or PR.** A new Go advisory (GO-2026-6859 today) is caught only when someone opens a PR. Add a weekly `schedule` to `ci.yaml` (or daily for `govulncheck` alone) so `main` fails on its own.
2. **The release does not run `govulncheck`.** `release.yaml` runs vet and test only, so a tag can publish a binary with a known vulnerability. v0.5.0 was built with Go 1.26.10, which `govulncheck` now reports (GO-2026-6859, reachable through `net/http` in `knowledge`). Add `govulncheck` before goreleaser.
3. **v0.5.0 binaries carry GO-2026-6859.** SECURITY.md promises a patch release for the latest minor. Decide whether to cut 0.5.1 (the toolchain fix is on `main` with the P13 work). Note: `main` has unreleased features, so a 0.5.1 from `main` ships them too; a branch from `v0.5.0` plus the go.mod line ships only the fix.

### Medium

4. **Cache in the release job.** `actions/setup-go` caches by default; a tag build restoring a cache written by another ref is the cache-poisoning path zizmor flags. Set `cache: false` in `release.yaml`.
5. **No build provenance.** Add `actions/attest-build-provenance` for `checksums.txt` and the archives (`id-token: write`, `attestations: write` on the release job), so users can run `gh attestation verify`.
6. **Release tags are unprotected.** No ruleset covers `refs/tags/v*`: a tag can be moved or deleted after release. Add a tag ruleset (deletion, update, non-fast-forward) for `v*`. Repository setting.
7. **CodeQL bumps arrive as two PRs** (#28 init, #29 analyze) that must move together. Merge both now; add a dependabot `groups` entry for `github/codeql-action/*`.

### Low

8. **SHA pinning is not enforced.** All actions are pinned, but `sha_pinning_required` is off; turning it on keeps it that way. Repository setting.
9. **No `timeout-minutes` and no `concurrency`.** A hung job runs for 6 hours; a new push to a PR does not cancel the old run. Add `timeout-minutes` per job and `concurrency: { group: ci-${{ github.ref }}, cancel-in-progress: ${{ github.event_name == 'pull_request' }} }`.
10. **Admin bypass is `always`.** The maintainer can push to `main` skipping every check. Single-maintainer repo, so acceptable; `bypass_mode: pull_request` keeps the escape hatch but only through a PR. Repository setting.
11. **Merged branches stay.** `delete_branch_on_merge` is off (about 25 old branches remain). Repository setting.
12. **AGENTS.md Verify lists only `go vet` and `go test`.** CI also requires `gofmt -l .` (PR #32 failed on it). Add it.

### Not needed

- A second secret scanner, Scorecard, or a workflow linter as a new CI dependency: the gaps above are few and specific; GitGuardian already overlaps GitHub secret scanning.
- `allow_rebase_merge` is on but unused; harmless.

## Proposed split

- **This repo, one PR (files):** 1, 2, 4, 5, 7 (dependabot groups), 9, 12.
- **Maintainer, settings:** 6, 8, 10, 11, and merging #28 and #29.
- **Maintainer, decision:** 3 (0.5.1 and from where).

## Verify

- Each workflow file parses as YAML, and every new action is pinned to a full SHA with its version in a comment.
- CI on the PR: every job green.
- `release.yaml` runs only on a tag, so its new steps are checked by reading: the `govulncheck` step is the same command as in `ci.yaml`, and the attestation step follows the action's documented inputs. The first real check is the next tag.

## Decisions (maintainer, 2026-11-16)

- Files, one PR: 1, 2, 4, 5, 7 (groups), 9, 12.
- 3: cut 0.5.1 from `v0.5.0` with only the toolchain line, after this PR, so the tag also builds with the hardened `release.yaml`.
- Settings 6, 8, 10, 11 and merging #28 and #29 stay with the maintainer.

## Result

- `ci.yaml`: daily `schedule` (05:17 UTC) runs `govulncheck` alone (`test`, `gofmt`, `race` skip on schedule); `timeout-minutes` per job; `concurrency` cancels an older PR run, and on `main` keeps one group per commit so no commit loses its status.
- `codeql.yml`: same `concurrency`, `timeout-minutes: 30`.
- `release.yaml`: `concurrency` per tag without cancelling; `timeout-minutes: 30`; `setup-go` `cache: false`; `govulncheck` before goreleaser; `actions/attest` v4.2.2 (pinned by SHA, released 2026-08-04) with `subject-checksums: ./dist/checksums.txt`, adding `id-token: write` and `attestations: write`. `actions/attest` instead of `attest-build-provenance`: the latter's README says v4 is a wrapper and new setups should use `actions/attest`. No `artifact-metadata` permission: storage records apply only with `push-to-registry`.
- `dependabot.yml`: group `codeql-action` for `github/codeql-action*`.
- AGENTS.md Verify: `gofmt -l .` first.
- Checked: every file parses as YAML; every `uses:` is pinned to a 40-character SHA with its version. `actionlint` is not installed here.
