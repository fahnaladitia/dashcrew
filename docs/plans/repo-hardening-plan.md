# Repo hardening plan

Tier: standard (repo files and CI only; no change to dashcrew's behaviour or generated output). Starts after P7.

## Findings (2 Nov)

- `govulncheck` reports 17 reachable Go standard library vulnerabilities when built with go1.26.3, fixed in go1.26.6. It also reports 5 vulnerabilities in required modules that the code does not call.
- `main` has no branch protection and no ruleset.
- Dependabot alerts and security updates are off.
- Third-party actions are pinned by tag, including `goreleaser/goreleaser-action@v6` in the release job that holds `HOMEBREW_TAP_TOKEN`.
- Community health is 42%: there is no SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md or issue template.
- CI runs `go vet` and `go test` only. Two files are not gofmt-clean.

Already fine: secret scanning and push protection are on; default workflow permissions are `read`; CI uses `pull_request`, not `pull_request_target`; the release runs only on tags pushed by writers.

## Repo changes (agent work)

1. `go.mod`: `toolchain go1.26.6` (or the newest patch release that is at least two weeks old).
2. `.github/dependabot.yml`: weekly updates for `gomod` and `github-actions`.
3. Workflows:
   - Pin every action to a commit SHA, with the tag in a comment.
   - Set `permissions: contents: read` explicitly in `ci.yaml`.
   - New CI jobs:
     - `govulncheck` (via `go run golang.org/x/vuln/cmd/govulncheck@<version>`);
     - `gofmt -l` (fails on output);
     - `go test -race ./...`;
     - a test matrix on `ubuntu-latest`, `macos-latest` and `windows-latest` (Windows may skip shim tests that need `sh`, with the reason in the skip message).
   - CodeQL for Go in its own workflow.
4. Fix the two files `gofmt` flags.
5. `SECURITY.md`: supported versions, private vulnerability reporting through GitHub, and a response window.
6. `CONTRIBUTING.md`: build, test, goldens (`DASHCREW_UPDATE_GOLDENS=1`), the commit pattern (`dashcrew commit lint`), and the rule that the repo never names a company or its domain.
7. `CODE_OF_CONDUCT.md`: Contributor Covenant 2.1.
8. `.github/ISSUE_TEMPLATE/` (bug, feature) and a pull request template for this repo.
9. Wiki skeleton as Markdown files outside the repo, for the maintainer to paste: Getting Started, Recipes, Team features, Migration, FAQ and troubleshooting, Design notes, Roadmap.

## GitHub settings (maintainer, or `gh api` only after an explicit yes)

- A ruleset on `main`: block force pushes and deletion, and require the CI checks on pull requests.
- Enable Dependabot alerts and Dependabot security updates.
- Enable private vulnerability reporting.
- Wikis: "Restrict editing to collaborators only".
- Commit email: use the GitHub noreply address for new commits, and turn on "Block command line pushes that expose my email". Existing history is not rewritten.

## Progress

- Repo changes done (2 Nov).
  - `toolchain go1.26.10`. govulncheck finds 0 reachable vulnerabilities. One unreached Windows-only issue remains in golang.org/x/sys v0.41.0, fixed in v0.44.0; it is left to Dependabot.
  - Actions are pinned to SHAs: checkout v5.1.0, setup-go v6.5.0, goreleaser-action v6.4.0, codeql-action v4.39.0. `persist-credentials: false` everywhere.
  - `ci.yaml` (`contents: read`):
    - `test` on ubuntu, macos and windows, with `fail-fast: false`. Windows has `continue-on-error`, and tests that need `sh` skip there with a reason;
    - `gofmt`, `race` and `govulncheck` jobs.
  - `codeql.yml`. `release.yaml`: `permissions: {}` at the top, `contents: write` only on the job.
  - Files: `.github/dependabot.yml`, SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, issue templates and a PR template. Two files reformatted.
  - Wiki skeleton (9 pages) written outside the repo for the maintainer.
  - Unverified until pushed: Windows CI, CodeQL, the Dependabot `cooldown` key, and the release workflow.
- GitHub settings: not done (the maintainer's).
