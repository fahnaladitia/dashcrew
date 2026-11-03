# P10b plan: personal profile (`me.yaml`)

Tier: large. Idea: `docs/ideas/knowledge-tracker-multirepo.md`, sections "Personal profile" and "Symlink layer". Runs after P10, so two stacks exist (Flutter and Go) and a role maps to different needs.

## Principle

The repo decides the rules; the person decides the experience. The guard never reads the profile, and a role grants no authority: a member editing a repo outside their role follows that repo's rules and guard exactly like its owners.

## What the agents support (checked 3 Nov 2026 in their docs)

| Agent | Personal instructions outside any repo | Personal skills |
|---|---|---|
| Claude Code | `~/.claude/rules/*.md` (user scope, loaded in every project, no approval dialog), under `$CLAUDE_CONFIG_DIR` when set. `CLAUDE.local.md` exists per repo, but makes Claude stop reading AGENTS.md by default, so it is not used. | `~/.claude/skills/<name>/`, which may be a symlink to a folder elsewhere |
| Codex | `~/.codex/AGENTS.md` (under `$CODEX_HOME`), concatenated before the repo's AGENTS.md. One file per level, so dashcrew owns a marked block in it. `AGENTS.override.md` replaces instead of adding, so it is not used. | not used in this phase |
| Antigravity | `~/.gemini/config/rules/*.md`, modular rule files with front matter, besides `~/.gemini/GEMINI.md` | not used in this phase |

User and project instructions are concatenated, not ranked; when they disagree the agent may follow either. The personal file therefore says exactly when it applies and never restates a repo rule.

## Design

- `~/.dashcrew/me.yaml`, one per machine, never committed:
  ```yaml
  role: mobile        # be | fe | mobile | devops | fullstack | lead
  stacks: [flutter]   # platform names; default from role
  level: regular      # new | regular
  chat_language: id   # ISO 639-1; empty keeps each repo's language.chat
  agents: [claude-code]
  ```
- `dashcrew me init [-p <repo>]`: proposes `stacks` from the platform of the repo it runs in, `role` from that platform (flutter: mobile, go: be, else fullstack) and `agents` from the config folders on the machine; asks to confirm each value, then writes the file. Run again in another repo to add its stack. `dashcrew me show` prints the profile (a branch command has no run of its own).
- `dashcrew me sync`: renders the personal instructions from embedded templates and writes them where each agent in `agents` reads them:
  - Claude Code: `<config dir>/rules/dashcrew-me.md`, a file dashcrew owns;
  - Codex: a block between `<!-- dashcrew:me:begin -->` and `<!-- dashcrew:me:end -->` in `<codex home>/AGENTS.md`, everything outside the block untouched;
  - Antigravity: `~/.gemini/config/rules/dashcrew-me.md`.

  A file dashcrew did not write is never overwritten. `dashcrew me sync --remove` takes all of it out.
- Effects in the personal instructions (all four confirmed):
  1. Outside the stacks: when the project's platform (`platform:` in `.ai/project.yaml`) is not one of the member's stacks, step 0 lists every assumption as a question, the reviewer always runs with the project's review skills, and explanations go one level deeper.
  2. `level: new`: before step 0, read the knowledge index and the pages for the task's area, and with `project.links` the cross-repo flow, then say in a few lines what was learned.
  3. `chat_language`: chat answers in that language, overriding the project's `language.chat` for this member only; code, docs and PR descriptions keep the project's languages.
  4. `role: lead`: prefer knowledge inbox review, release status, audit and PR review; when asked for code, the chain runs as usual.
- Personal skills: `dashcrew-onboarding` (with `level: new`) and `dashcrew-lead` (with `role: lead`), prefixed so they never collide with a repo's or the user's own skills,, shipped embedded, written to `~/.dashcrew/skills/<name>/` and linked into `<config dir>/skills/<name>` for Claude Code. Only the ones the profile selects are linked; a link dashcrew did not make is never replaced.
- `doctor` reports `me`: profile valid, personal files in step with it, links pointing at existing folders.

## Stages

- M0: profile format, parser, `dashcrew me`, `dashcrew me init`.
- M1: personal instruction templates, `dashcrew me sync` for the three agents, ownership markers, `--remove`.
- M2: personal skills and their links.
- M3: `doctor` check, README, live check on this machine (Claude Code under `CLAUDE_CONFIG_DIR`).

## Not doing

- Any rule that changes what the guard allows.
- Per-repo personal files (`CLAUDE.local.md`, `AGENTS.override.md`).
- Personal skills for Codex and Antigravity.
- Reading history across several repos in `me init`.

## Decisions (all confirmed)

1. Personal skills: `onboarding` and `lead`.
2. `me init` reads the current repo only.
3. Codex: a dashcrew block in `~/.codex/AGENTS.md`.
4. Effects: all four in the first version (2 Nov).

## Progress

- [x] M0 profile and init (`internal/me/profile.go`, `propose.go`, `cmd/dashcrew/me.go`)
- [x] M1 personal instructions and sync (`personal/rules.md`, `internal/me/sync.go`)
- [x] M2 personal skills (`personal/skills/`, symlink with a marked copy as fallback)
- [x] M3 doctor `me` check (opt-in field the CLI sets), README, CHANGELOG
- [x] Live check (3 Nov) with a temporary HOME and `CLAUDE_CONFIG_DIR` on a Go sample service: init, sync, doctor, a user Codex file kept, a foreign skill folder kept, level and stack changes, remove. The real config was left alone at the user's choice. Released in 0.3.0.
