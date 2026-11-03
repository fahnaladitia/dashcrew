# P10c plan: role and level per project (hybrid profile)

Tier: standard. Follows P10b (`~/.dashcrew/me.yaml`, released in 0.3.0).

## Problem

P10b gives one role and one level per machine. A member who leads one repo and is a regular member of another, or who is new to one repo and not to the others, cannot say so.

## Principle (unchanged)

The repo decides the rules; the person decides the experience. The guard never reads either profile, a role grants no authority, and nothing personal is ever committed.

## Design: hybrid (confirmed 3 Nov)

| Data | What it describes | Where |
|---|---|---|
| `stacks`, `chat_language`, `agents` | the person, the same in every repo | `~/.dashcrew/me.yaml`, per machine |
| `role`, `level` | the person's place in one team | `<repo>/.ai/me.yaml`, never committed; the home values are the default for a repo without one |

- `.ai/me.yaml` holds only `role` and `level`, and only the ones that differ from home; any other key is an error that names the home file. When init's answers match home, the repo file and its personal files are removed and the repo forgotten.
- A repo without `.ai/me.yaml` behaves as in 0.3.0.
- One command inside a repo: `dashcrew me init`
  - no home profile yet: asks `stacks` (proposed from the repo's platform), `chat_language`, `agents` once and writes the home file, with `role`/`level` there as defaults;
  - then asks `role` and `level` for this repo (proposed from the home defaults), writes `.ai/me.yaml`, excludes it from git, and runs `me sync`;
  - outside a dashcrew repo, or with `--home`, it only writes the home file (the 0.3.0 behaviour).
- Never committed: the repo's personal files are listed in `.git/info/exclude` (decision 2). `doctor` fails the `me` check when git tracks any of them.

## What the agents read per repo (checked 3 Nov 2026 in their docs)

| Agent | Per-repo personal file | Notes |
|---|---|---|
| Claude Code | `<repo>/.claude/rules/dashcrew-me.md` | Rules without `paths:` load at launch, same priority as `.claude/CLAUDE.md`, no approval dialog; user rules load first, both are concatenated. `CLAUDE.local.md` is not used: it counts as a CLAUDE.md, so Claude stops reading AGENTS.md by default. A git-ignored file exists only in the worktree it was made in. |
| Antigravity | `<repo>/.agents/rules/dashcrew-me.md`, `trigger: always_on` | Workspace rules folder. One unofficial report says rules listed in `.gitignore` are not loaded; `.git/info/exclude` works. |
| Codex | none | One instruction file per folder, and the repo root already has AGENTS.md. The repo's lines go in the dashcrew block of the user AGENTS.md, keyed by the repo path (decision 3). |

## Sync

- `dashcrew me sync` keeps writing the home files (P10b). Run inside a repo with `.ai/me.yaml` it also writes:
  - Claude Code and Antigravity: the repo rule file, holding only the `role`/`level` lines for this repo ("In this repository your role is lead ..."), plus the lead and new-member sections they turn on or off;
  - Codex: a section "In the repository at `<path>`: ..." with the same lines.
- The home instruction file gains one line: a repo's own `dashcrew-me` rule (or Codex section) decides role and level in that repo. Agents concatenate both, so the line says which wins.
- `me sync --remove` inside a repo removes that repo's files and its Codex section; at home level (`--home`) it removes everything, as in 0.3.0.
- Personal skills stay user-level: `dashcrew-lead` / `dashcrew-onboarding` are linked when the home profile or any synced repo selects them; a repo rule names a skill only when that repo selects it.
- Worktrees share the main checkout's personal files: `worktree create` and `worktree link` symlink every toolkit path the main checkout ignores.

## Doctor

`me` check: effective role and level with their source (repo or home), files out of step, Codex sections whose repo path is gone, and personal files tracked by git (fail).

## Stages

- C0: `.ai/me.yaml` format (role, level only), effective profile, exclude entries, `me init` two-layer flow, `me show` with sources.
- C1: repo sync for Claude Code and Antigravity, the Codex section, `--remove` per repo, the deciding line in the home file.
- C2: skills selected by any synced repo; `worktree create` copy.
- C3: doctor, README, CHANGELOG, live check with a temporary HOME on two sample repos (lead in one, new in the other).

## Edge cases

- A repo that commits its own `.claude/rules/`: only `dashcrew-me.md` is personal; a committed file of that name is never overwritten (kept, exit 1).
- A repo moved or renamed: its Codex section goes stale; `me show` and `doctor` list it, `me sync --prune` drops it.
- `.ai/me.yaml` written by hand with other keys: refused, naming `~/.dashcrew/me.yaml` as their place.

## Not doing

- Any guard change, and anything committed to a repo.
- `CLAUDE.local.md`, `AGENTS.override.md`, Codex project fallback names.
- `stacks`, `chat_language` or `agents` per repo.
- Matching repos by git remote (path only in this phase).

## Decisions

1. Hybrid: per machine `stacks`/`chat_language`/`agents`, per repo `role`/`level` (confirmed 3 Nov).
2. Ignore mechanism: `.git/info/exclude`, written per clone, no diff (confirmed 3 Nov).
3. Codex per repo: a path-keyed section in the dashcrew block of the user AGENTS.md (confirmed 3 Nov).

## Progress

- [x] C0 `internal/me/repo.go` (role and level only, overrides of home), `me init` two layers, `me show` with sources
- [x] C1 target model in `internal/me/sync.go`: home and every registered repo (`~/.dashcrew/me-repos`) in one sync; repo rules for Claude Code and Antigravity, Codex sections, `--remove-repo`, `--prune`; a repo without `.ai/me.yaml` gets its rules removed
- [x] C2 skills from any repo. Worktrees: nothing new, since `worktree create`/`worktree link` already symlink every toolkit path the main checkout ignores, the personal files included (found in the live check; a copy plus a separate registration was dropped)
- [x] C3 doctor (effective role and level, tracked personal file fails), README, CHANGELOG
- [x] Live check (3 Nov), temporary HOME: a copy of the Go sample service as lead, a new Flutter repo as new member, all three agents; doctor shows the source of each value and the platform outside the stacks; Codex got one section per repo; git status shows no personal file; a task worktree links the personal files from the main checkout; `--remove` cleans home and both repos.
