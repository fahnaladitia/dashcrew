# Release checklist

For the maintainer, in order. Each step is a yes or a no; a no stops the release until it is fixed.

## 1. Before any tag

- [ ] `main` is green in CI, including `release-snapshot`, `upgrade` and `staticcheck`.
- [ ] CHANGELOG: "Unreleased" becomes the version heading.
  - Entries are written from the user's side, with no plan codes (P3, P10b).
  - The first paragraph says whether a project must run `dashcrew sync` and commit.
  - It also says whether `guard_min_version` rose, in which case every teammate installs the new binary before that sync is committed.
- [ ] Decide whether the release needs a release candidate. It does when generated files, the lock, the guard policy or `guard_min_version` change; otherwise go to step 3.

## 2. Release candidate

- [ ] Tag `vX.Y.Z-rc.N` and push the tag. It is published as a GitHub pre-release and is not pushed to the Homebrew tap. Install it with `go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@vX.Y.Z-rc.N`.
- [ ] Live run on one real project per agent:
  - [ ] `dashcrew sync` on a project synced by the previous release: no `--force` needed.
  - [ ] `dashcrew doctor`: no `fail`.
  - [ ] A real session in Claude Code, in Codex and in Antigravity, each doing one small task through the chain.
  - [ ] In each session, ask the agent to run `git push`: the guard blocks it with its reason.
  - [ ] `dashcrew check` on a PR branch passes.
- [ ] Write down what broke. Anything that broke is fixed and tagged as the next rc, and the live run starts over.
- [ ] The rc stays with the office team for at least 2 working days with no new problem.

## 3. Final tag

- [ ] Tag `vX.Y.Z` on the commit the last rc was cut from, or on `main` for a release without an rc, and push the tag.
- [ ] The release workflow is green: vet, test, govulncheck, goreleaser and attest.

## 4. After the release

- [ ] `brew update && brew upgrade dashcrew && dashcrew --version` prints `X.Y.Z`.
- [ ] `go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@vX.Y.Z && dashcrew --version` prints `X.Y.Z`.
- [ ] `gh attestation verify <one archive> -R fahnaladitia/dashcrew` passes.
- [ ] The install lines in the README still work as written.
- [ ] Tell the office team, quoting the CHANGELOG's first paragraph.
