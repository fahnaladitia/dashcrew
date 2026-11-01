# Shared knowledge, tracker status, multi-repo tracks and a second platform (P5-P8)

Follows `docs/plans/p4-skill-first-plan.md`. Each phase gets its own plan in `docs/plans/` before any code.

Audience: internal team use first. Platforms: stack-agnostic, with Flutter as the built-in reference and any other stack through levels 1-2 (P4 S0). Name: dashcrew. Order: P4 -> P5 knowledge -> P8 second platform -> P7 tracks -> P6 tracker check. P8 comes before P7 because cross-repo tracks pay off most when a second stack is in.

## Problem Statement

How might a small team whose members each use their own AI agent:
- share what they learn about the product, so a new member or any agent can ask "how does checkout work?" and get an answer that is current and checked against the code;
- see who is working on what without running a server;
- work on a task that spans several repos without stashing or switching branches in their main clones?

## Recommended Direction

**P5: shared knowledge repo.**
- One git repo for the team, separate from the app repos.
- It holds feature pages (`features/<slug>.md`: purpose, entry points per repo, state holders, endpoints, decisions and why with PR links, gotchas) and repo facts (`repos/<repo>.md`).
- Every fact carries a `<!-- verified <repo>@<sha> <date> -->` marker.
- How knowledge gets in:
  1. The retro at the end of a task (the `learn` workflow) writes proposals to `inbox/` in the local clone of the knowledge repo.
  2. `dashcrew knowledge review` walks the inbox: accept, edit, skip, delete. Accepting merges the proposal into the feature page; it does not append a new note.
  3. The member commits and opens a PR on the knowledge repo. PR review is the team's curation.
- `dashcrew knowledge doctor` flags a fact as stale when a path it names no longer exists at the repo's HEAD.
- `sync` registers an MCP server `dashcrew knowledge serve` through the existing owned-MCP handling, with three tools:
  - `knowledge_search(query, repo?)` returns ranked snippets with page paths;
  - `knowledge_read(page)` returns a page;
  - `knowledge_sources(page)` returns its `file:line` pointers, so the agent re-reads the code before it answers.
- Search is an in-memory BM25 index built when the server starts, so there is no database. A few hundred pages index in well under a second.
- The catalog ("knowledge book") is a generated `README.md` index in the knowledge repo, rendered by GitHub. A static site comes only if that proves too thin.
- AGENTS.md rule: before planning, search knowledge for the feature; the `explorer` searches knowledge before it sweeps code.
- Onboarding: a new member asks the agent; the agent answers from knowledge and verifies against the code.

**P6: tracker as the team view.**
- The ClickUp pack already moves a task to `in progress` when its branch is created, and to `in review` with the PR link when the PR exists.
- P6 checks that flow live, makes it an explicit chain step in the P4 chain, and adds a GitHub Issues/Projects tracker pack for teams without ClickUp.
- It shares status only, never transcripts.
- It still runs only when the work started from a tracker task.

**P7: multi-repo tracks in dashcrew.**
- A workspace layer above the per-repo toolkit:
  ```
  <workspace>/repos/<repo>/        main clones, never edited
  <workspace>/tracks/<task>/<repo> one worktree per repo, same branch name
  <workspace>/tracks/<task>/AGENTS.md  repos, branch, merge order, overrides
  ```
- `dashcrew task new <task> core,app_a [--from <ref>]` creates the worktrees, runs `pub get` in each, and writes a git-ignored `pubspec_overrides.yaml` in each app that points at the core worktree. `dashcrew task ls` and `dashcrew task done` complete the set.
- Each repo's own committed dashcrew output (AGENTS.md, skills, guard) still applies inside its worktree. The track AGENTS.md only adds the cross-repo facts.
- PRs are merged core first. `dashcrew check` refuses an app PR that still depends on an unmerged core branch.
- Commit and push stay with the user. The CLI may run them when the user runs the CLI; the guard blocks agents from running those subcommands.
- A one-repo track replaces today's `worktree` workflow, so single-repo parallel work and multi-repo work become the same thing.

**P8: the second platform, whatever stack the team adopts.**

P4 S0 makes core platform-agnostic. Flutter is the built-in reference platform; any stack enters through three levels:

1. **`platform: other`:** the team declares the commands in the profile and gets all of core on day one.
2. **Project platform:** the team writes `.ai/platforms/<name>/` in its own repo from its real code.
3. **Built-in:** a project platform that proves generic moves into this repo.

P8 takes the team's next stack through levels 1 and 2. Go is the current candidate; the same path applies if the team moves to another stack.

What a platform at level 2 should cover, shown for Go as the worked example:

- `platform.yaml`:
  - `detect: [go.mod]`;
  - `commands`: `go vet ./...`; `go test ./...` and its scoped form `go test ./<pkg>/... -run <Test>`; `go mod download`;
  - the linter only if the repo already configures one;
  - `link`: `go.work`, the counterpart of `pubspec_overrides.yaml`, so P7 tracks join repos.
- Rules, with the same shape as Flutter's:
  - layered flow `handler -> service -> repository -> store`, with names taken from the team's repos;
  - table-driven tests with hand-written fakes behind interfaces;
  - `context.Context` first and passed down;
  - errors wrapped with `%w` and never swallowed;
  - goroutines owned and stopped (the counterpart of Flutter's dispose rule);
  - doc comments on exported identifiers.
- Families, only the ones the team's repos use. Go candidates: `http` (net/http, chi, gin, echo), `db` (database/sql, sqlx, pgx, sqlc, gorm), `migrate` (goose, golang-migrate, atlas), `di` (manual, wire, fx), `log` (slog, zap). `tracker` is shared across platforms.
- Guard additions a backend platform contributes:
  - block migration commands against non-local databases;
  - block `DROP` and `TRUNCATE` through database CLIs;
  - block deploy and infra apply commands from agents.
- `review: true` skills: SQL injection and query safety, auth and permission checks on handlers, secrets in config, migration reversibility.

Team-specific conventions stay in the team's private project platform. This repo receives a platform at level 3 only when it holds conventions any team on that stack would accept.

**Personal profile (`me.yaml`).**

The repo decides the rules; the person decides the experience. A member editing a repo outside their role follows that repo's rules and guard exactly like its owners. The profile is self-declared, so it grants no authority: the guard never reads it.

- One file per machine, never committed, because one person works across many repos:
  ```yaml
  # ~/.dashcrew/me.yaml
  role: fe            # be | fe | mobile | devops | fullstack | lead
  stacks: [react]     # optional, derived from role
  level: new          # new | regular
  chat_language: id
  ```
- `dashcrew me init` proposes a role from the git log (the repos and paths the member commits to most); the member confirms or edits it.
- Effects:

  | Profile | What changes |
  |---|---|
  | `role` with a stack | Only that stack's skills are linked on this machine, so the skill index and the context stay small |
  | Working outside the role (for example, `fe` in an infra repo) | Step 0 asks more, the reviewer always loads that repo's review checklist, and explanations go into more detail |
  | `fullstack` | All of the team's stacks; multi-repo tracks are the default flow |
  | `lead` | Knowledge inbox review, a team status summary from the tracker, `audit` and PR review, rather than code |
  | `level: new` | Onboarding mode: before a task, the agent walks the relevant knowledge pages and the cross-repo flow |

**Symlink layer.**

What binds the team is committed. What is personal or changes often is linked.

| Layer | How | Why |
|---|---|---|
| AGENTS.md, guard policy, team skills pinned in the lock | Committed, generated as today | The alignment layer. It must be visible in PRs and checked by `dashcrew check` in CI. A link to a machine path is invisible to both. |
| Role skills, `~/.dashcrew/skills/<stack>/` | Symlinked into each agent's skills folder, git-ignored | One update reaches every repo on the machine; no per-repo copies. Selected by `me.yaml`. |
| Shared knowledge repo | Symlink `.ai/knowledge -> ~/.dashcrew/knowledge/<team>` | Agents read and grep knowledge as plain files, cheaper than MCP round trips. The MCP stays for ranked search and for agents without file access. |
| Worktrees and tracks | Links recreated by `dashcrew task new` | A new worktree does not carry git-ignored files. |

Design requirements for the symlink layer:

1. **The guard resolves the real path** before applying any write rule. Through the knowledge link, only `inbox/` is writable, so an agent cannot edit a reviewed page by writing `.ai/knowledge/features/x.md`.
2. **Link targets are recorded in the lock.** This means the knowledge sha and the role-skill version. `doctor` reports drift, for example "knowledge is 40 commits behind origin".
3. **Windows falls back to a copy** when symlinks are unavailable (no Developer Mode, or `core.symlinks=false`). The lock marks the entry as copied, and `sync` refreshes it.
4. **Each agent is checked live.** Confirm it follows symlinked skill folders before relying on it. Claude Code is expected to; Codex and Antigravity are unverified.
5. **`.ai/` is excluded** in the analyzer, test and indexing configuration that dashcrew generates, so tools do not walk into linked folders.

## Key Assumptions to Validate

- [ ] Retro proposals are worth curating. Run P5 on five real tasks and count accepted proposals and the edits each one needed.
- [ ] Knowledge answers beat reading the code cold. Ask the same three onboarding questions with and without the knowledge MCP and compare correctness and tokens.
- [ ] In-memory BM25 finds the right page. Measure top-3 hit rate on twenty real questions before considering embeddings or SQLite.
- [ ] The team will review knowledge PRs. If proposals pile up unreviewed for two weeks, the curation step needs a different owner.
- [ ] The core repo split will happen. Get a date before building P7.
- [ ] A role-trimmed skill set does not miss skills a task needed. Compare the step 0 `Skills:` lines of ten tasks with and without the profile.
- [ ] Codex and Antigravity load skills through symlinked folders. Check each live before the symlink layer ships.
- [ ] Level 1 is useful on its own. Run one real task on a non-Flutter repo with only `platform: other`, and list which missing rules the reviewer had to catch by hand. That list is the first content of the level 2 platform.
- [ ] The P8 platform's conventions match the team's repos. Read two real repos of that stack before writing a rule.

## MVP Scope

- **In P5:**
  - knowledge repo layout and profile key `knowledge: { repo, path }`;
  - retro proposals to `inbox/`;
  - `knowledge review`, `knowledge doctor` and `knowledge serve` (MCP, three tools);
  - the generated index README;
  - the AGENTS.md rule.
- **In P6:** live check of the ClickUp flow, the flow as a chain step, a GitHub tracker pack.
- **In P7:** workspace, tracks, overrides, merge-order check, one-repo track replacing `worktree`.

## Not Doing (and Why)

- **A hosted website or monitor dashboard:** the tracker board already shows who is on what; a server adds auth, hosting and data to protect.
- **A database as the source of truth:** Markdown in git is reviewable, diffable and synced by the tools the team already uses. Any index is derived and rebuildable.
- **Fully automatic knowledge writes:** unreviewed knowledge goes stale and an MCP answering confidently from it is worse than no answer.
- **Embeddings in the MVP:** only if BM25 misses too often on real questions.
- **Sharing agent transcripts:** status yes, transcripts no.
- **Moving rules into the knowledge repo:** rules stay in AGENTS.md and project packs. Knowledge holds facts and flows. A retro proposal that is really a rule is routed to the project pack instead.

## Open Questions

- Knowledge repo access: one repo for the whole team, or one per product when several products share members?
- Where the local clone lives: under `~/.dashcrew/knowledge/<name>` (proposed), or next to the workspace in P7?
- How `knowledge serve` notices a pull: rebuild the index on every call when HEAD moved (proposed), or watch the folder?
- P7 base per repo: one `--from` for all repos, or per repo like `core=origin/develop,app_a=origin/main`?
- Personal profile and symlink layer: their own phase after P5, or folded into P5? Knowledge linking needs the symlink layer, and role skills need stack packs beyond the reference stack.
- `me.yaml` per repo override (for example, `lead` in one repo, `fe` in another): needed, or one role per machine is enough?
