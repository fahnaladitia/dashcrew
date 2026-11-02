# P8 plan: bridge to other team tools

Tier: large (a public interop contract, a new top-level `status` command, a new `bridge` command family with a first adapter, a guard change, doctor checks, one new chain instruction). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P8. Builds on P5 (knowledge repo, inbox, verified markers), P5b (`verify record`), P6 (`dashcrew-release`) and P7 (`dashcrew-spec`).

## Goal

A team where some members also use another agent tool can use both without either tool overwriting the other, and without knowledge or status living in two places that drift apart. dashcrew keeps its own formats. A bridge is a translator run on demand: no server, no long-running process, no shared database.

The first adapter targets siding (github.com/rans0/siding, MIT, Go): a workspace manager that gives each task a track (one git worktree per repo, a shared branch, a generated `AGENTS.md`, two human gates). It is the closest tool to dashcrew in scope, and its file formats are public, so the knowledge and status parts of this plan need no change on its side.

## What exists

- **dashcrew:** knowledge repo with `features/`, `repos/`, `inbox/`; proposals with front matter `kind, target, repo, branch, commit` (unknown keys rejected); verified marker `<!-- verified <repo>@<sha> <yyyy-mm-dd> -->`. Branch config `dashcrew-release` and `dashcrew-spec`. Verify record at `<git dir>/dashcrew/last-verify`. `release status` and `spec status` exist; there is no top-level `status`.
- **Guard:** policy is `<hook cwd>/.ai/guard.json`. With no policy there, every call is allowed, silently.
- **siding** (read at a2f108e, 30 Oct 2026):
  - Workspace: `siding.toml`, `repos/` (main clones), `tracks/<task>/` (`AGENTS.md`, `.track.json`, one worktree per repo), `docs/<date>_<task>/`, `knowledge/{rules,skills,repos,inbox}`.
  - `.track.json`: `task, hotfix, created, docs, status, slot, repos[name, branch, base, base_commit, head], review, adopt, log_sum`.
  - Inbox proposal: `key: value` front matter `repo` (a repo, `rule:<name>` or `skill:<name>`), `track`, `date`, `commit`; unknown keys ignored.
  - Accepted fact: one bullet in `knowledge/repos/<repo>.md` ending `<!-- verified <commit> <track> <date> -->`. Every track touching that repo gets the file in its `AGENTS.md`.
  - Per track it writes `.claude/settings.json`, `.codex/hooks.json`, `.cursor/hooks.json` or `.opencode/plugins/` with its own guard (`siding guard`), regenerated on track changes.

## The coexistence gap (found while reading)

siding starts the agent in `tracks/<task>/`, one level above the worktrees. dashcrew's files live inside each worktree. So in a siding track, unless something changes:

- the dashcrew guard does not run: the harness loads hooks from the track folder, where only siding's guard is registered; and `dashcrew guard` run from the track folder finds no `.ai/guard.json` and allows everything;
- the repo's `AGENTS.md` is not loaded at start (Codex reads from the git root down to the working folder; Claude Code loads nested instruction files only when it reads files in that folder);
- two workflows compete: siding's stages and gates (prd, Gate 1, build, Gate 2, ship) and dashcrew's chain (step 0, plan, tdd, review, verify, PR description, commit command);
- `siding pr` commits with a title from `prd.md`, which `dashcrew check` may reject under the team's commit pattern.

The harness loading behaviour is checked live in B5 before any fix depends on it.

## Stages

### B0: interop contract

- `docs/interop.md`, versioned (`interop: 1`): the knowledge page, the inbox proposal, the verified marker, the code pointer, the task status JSON (B1), and guard policy discovery (B5).
- Each format names the dashcrew function that reads it and a fixture under `internal/bridge/testdata/interop/` that the tests parse, so the document cannot drift from the code.
- Rule for later changes: adding a field is a minor change; renaming or removing one bumps `interop`.

### B1: `dashcrew status [--json]`

- Read-only. For the repo or worktree at `-p`, prints:
  - `branch`, `release` (`dashcrew-release`), `spec` (`dashcrew-spec`), `tier` (`dashcrew-tier`, see below);
  - `plan`: the plan path when it exists;
  - `verify`: `none`, `current` (recorded tree equals the working tree) or `stale` with the count of files changed since;
  - `pr_description`: the path when the file exists;
  - `step`: the furthest of `branch -> planned -> verified -> described -> pr`, derived from the facts above, never stored;
  - `pr`: number and state, only with `--pr` (calls `gh`; absent `gh` leaves it empty, not an error).
- New chain instruction at step 1: record the confirmed tier with `git config branch.<branch>.dashcrew-tier <tier>`, the same way release and spec are recorded. The guard already allows `git config branch.*` writes; checked in tests.
- `--json` output is the interop status object. Text output is one line per field.

### B2: bridge framework

- `internal/bridge`: an adapter interface (`Name`, `Detect(dir)`, `ImportKnowledge`, `ExportKnowledge`, `Status`), a registry, and the shared write rules enforced in code, not only in docs:
  - import writes only into dashcrew's own formats (the knowledge clone's `inbox/`);
  - export writes only under the folder given with `--to`, never overwrites an existing file without `--force`;
  - files another tool owns are read only. Each adapter declares its owned paths, and a test runs every adapter against a fixture workspace and asserts none of them changed.
- `dashcrew bridge list`: adapters, and for each whether it detects a workspace at `-p` or above.

### B3: siding knowledge, both ways through the receiver's inbox

- `dashcrew bridge siding import knowledge --from <siding workspace>`:
  - reads accepted facts in `knowledge/repos/<repo>.md` and writes one dashcrew proposal per fact into the knowledge clone's `inbox/` (`kind: repo`, `target: <repo>`, `repo`, `branch: <track>`, `commit` from the siding marker);
  - optional `--inbox` also carries siding's unreviewed proposals for repos (not `rule:` or `skill:`, which are agent instructions dashcrew's catalog owns; they are listed as skipped);
  - skips a fact whose normalised text already appears in the target page or in `inbox/`, so a second run adds nothing.
- `dashcrew bridge siding export knowledge --to <siding workspace>`:
  - writes siding inbox proposals (`knowledge/inbox/<date>-dashcrew-<slug>.md`, front matter `repo, track: dashcrew, date, commit`), one per fact paragraph of `repos/<repo>.md` and of each `features/<slug>.md` whose `repos` lists the repo; code pointers `<repo>:<path>:<line>` become backticked paths so siding's stale check can read them;
  - never touches `knowledge/repos/`, `rules/` or `skills/`; the user accepts with `siding learn` as usual.
- Neither direction writes a reviewed page. Knowledge crosses only as proposals, so each tool's own review stays the gate.

### B4: siding status

- `dashcrew bridge siding status --from <siding workspace> [--json]`: for each `tracks/*/.track.json`, the task, siding stage, and the B1 status of each worktree. Read only; it never calls `siding`.

### B5: coexistence in one workspace

- Live check first, in a scratch siding workspace with one dashcrew repo: which hook files Claude Code and Codex load when started in `tracks/<task>/`, and whether the repo's `AGENTS.md` is read. The findings go into `docs/interop.md`.
- Guard, workspace mode: when the hook's working folder has no `.ai/guard.json`, the guard looks one level down for `*/.ai/guard.json` and enforces the union of those policies (the strictest wins on every key). Paths are still resolved on the real path. No policy found at either level keeps today's behaviour. Generic: it applies to any folder that holds several dashcrew worktrees, siding's or a future P9 track.
- `doctor` inside a siding track or workspace (detected by `.track.json` in the parent or `siding.toml` above):
  - warns when the track folder's hook files do not run `dashcrew guard`, and prints the exact entry to add and where;
  - warns about the two workflows and the commit pattern, pointing at the wiki page.
- Wiki page "Using dashcrew with siding", and a short note in the README's tools section.

## Needs agreement with siding's author (not in P8)

- siding chaining `dashcrew guard` in the hook files it generates when a worktree holds `.ai/bin/guard`, and pointing its track `AGENTS.md` at each worktree's `AGENTS.md`.
- Which tool leads inside a track (see decision 1), written into both tools' docs.
- P9: whether multi-repo tracks become an adapter to siding instead of a dashcrew feature.

dashcrew opens an issue on siding with the B5 findings and a proposed change only after the user says yes.

## How we check

- B0: every fixture in `docs/interop.md` parses with the named function.
- B1: transcript tests for each `step` value; `verify` current and stale; `--pr` without `gh`.
- B2: owned-path test over every adapter; `--to` existing file without `--force` refused.
- B3: round trip on a fixture workspace: siding fact -> dashcrew proposal -> `knowledge review` accept -> export -> siding proposal that `siding learn` accepts (real siding binary in a manual check, fixture parser in tests). Second import adds nothing.
- B4: fixture `.track.json` files, including review and adopt tracks.
- B5: guard differential over the corpus is unchanged with a policy at cwd; new cases for workspace mode; doctor transcripts.
- Grep before every commit: the public repo names siding only as a tool, with no team or product names.

## Not doing

- Writing any file siding owns, or calling `siding approve`, `siding pr` or `siding done`.
- Importing siding rules or skills into dashcrew's catalog.
- A shared database, a sync daemon, or MCP between tools. MCP stays agent-facing (`knowledge serve` indexes imported knowledge once it is accepted).
- A second adapter. The framework allows one; it waits for a real second tool.
- Tracker sync (P11).

## Status

Deferred. The plan is kept as written; no stage has started.

## Decisions

Confirmed:

1. Inside a siding track, siding leads stages and gates; dashcrew stays the repo layer (rules, guard, knowledge, specs, commit pattern, release lines), and its chain maps onto siding's stages instead of running beside them.
2. Knowledge crosses only as inbox proposals on the receiving side.
3. The tier is persisted at step 1 as `dashcrew-tier`.

Open:

4. Is siding the tool the bridge is meant for, and may the public repo name it?

## Progress

- [ ] B0 interop contract
- [ ] B1 status
- [ ] B2 bridge framework
- [ ] B3 siding knowledge
- [ ] B4 siding status
- [ ] B5 coexistence
