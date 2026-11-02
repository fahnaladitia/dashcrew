# Roadmap after P4: knowledge, commits, release lines, specs, bridge, tracks, platforms, tracker (P5-P11)

Follows `docs/plans/p4-skill-first-plan.md` and `docs/plans/p4b-go-engine-plan.md`. Each phase gets its own plan in `docs/plans/` before any code.

Audience: internal team use first. Platforms: stack-agnostic, with Flutter as the built-in reference and any other stack through levels 1-2 (P4 S0). Name: dashcrew.

| Phase | Content |
|---|---|
| P5 | Shared knowledge (in progress) |
| P5b | Commit commands: one commit pattern for the team, a ready command at the end of every task |
| P6 | Release lines: parallel release branches and fix ports |
| P7 | Specs: PRD and TRD written and reviewed up front, kept in step during the work |
| P8 | Bridge to other team tools |
| P9 | Multi-repo tracks, or an adapter to another tool that already does them (decided after P8) |
| P10 | Second platform |
| P11 | Tracker check |

P5b, P6 and P7 come before the bridge because they solve daily work and depend on no one else. The bridge waits on agreement with the other tool's author. P5b is small and lands right after P5.

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

**P5b: commit commands.**

Agents never commit; the member does. Each member still writes messages a little differently, and after a manual edit the command the agent gave no longer matches the tree. So at the end of every task the agent hands over a ready command in the team's pattern, and regenerates it on request.

- Profile `project.commit`:
  - `pattern` (default `<type>(<scope>): <subject>`);
  - allowed `types`;
  - `scopes` mapped from path globs;
  - `ticket` taken from the branch name by a pattern;
  - `subject_max`;
  - the commit language;
  - optional body sections;
  - trailers such as `Refs:` and, with P6, `Ported-from:`.
  - Never AI attribution.
- `dashcrew commit suggest`: reads the current tree and the branch, then prints:
  - the derived parts: type from the branch prefix, scope from the changed paths, ticket from the branch name;
  - the files changed since the last verify;
  - the exact command, with an explicit file list (`git add <files> && git commit -m ...`), never `git add -A`. Stray files stay out, and new untracked files are listed for the member to decide.

  The agent writes only the subject and body, inside that frame.
- `dashcrew commit lint <message|--range a..b>`: checks a message, or every commit on a branch, against the pattern. `dashcrew check` runs it on the PR's commits.
- Chain: a step after the PR description, "commit command". The agent prints the block and never runs it.
  - When the member says they edited files by hand, the agent runs `commit suggest` again and gives a new command.
  - When the tree differs from what `verify` last checked, it says so and offers a scoped re-verify first.
- The guard keeps blocking `git commit` and `git push` for agents. `commit suggest` and `lint` are read-only, so agents may run them.

**P6: release lines.**

Teams often build two releases at once (for example `x.2.0` and `x.3.0`) and insert a patch line (`x.2.5`) while both are open. That affects four things:

- **Generated files merge across lines.** AGENTS.md, agent files, the lock and the shim are committed on every line, so merges between lines can conflict in them. The rule is that generated files are never merged by hand: take either side, run `dashcrew sync`, and commit. `doctor` and `check` already detect a tree that does not match its lock; they gain a hint to re-sync after a merge between lines.
- **Toolkit version per line.** Toolkit upgrades happen only on the main development line and reach other lines through normal merges. On a branch matching a release or hotfix pattern, `sync` refuses to upgrade the toolkit unless `--upgrade` is given, so a patch PR carries only the fix. The binary must keep working with older locks.
- **Fix ports.** The default direction is one way, upward: a fix lands on the oldest affected line and flows to newer lines and the main line by merge (`x.2.5 -> x.3.0 -> main line`). Cherry-pick is used only when merging upward would bring unwanted changes. Merging the main line down into a release line happens only when the line is created; two-way merges are where renames break and fixes get lost.
  - Workflow `port`: given a merged PR or commit range on one line and the target lines, it opens one worktree per target, merges or cherry-picks with rename detection, runs scoped verify, and writes one PR per target that names the origin PR.
  - `dashcrew check --ports`: compares the fixes marked on one line (trailer `Ported-from:` or patch-id) with another line, and reports fixes not yet ported or lost to a revert or a merge.
- **Knowledge per version.** Pages describe the main line. Version-specific parts are tagged in place, such as `(since x.3.0)` or `(x.2 only)`. There are no folders per version, because most knowledge holds across versions. The staleness check runs against the main line.

Grouping by version, decided:

| What | Grouped by version? |
|---|---|
| Plans and task documents: `<plans>/<release>/<task>/` | Yes. Step 0 gains one `Release:` line. |
| A generated release ledger: `<plans>/<release>/README.md` (tasks, PRs, pending ports) | Yes. `check --ports` reads it. |
| Application code | No. Branches carry the versions. |
| Generated toolkit files | No. There is one set per branch, decided by the lock. |
| Knowledge | No. Tag the version-specific parts instead. |

Profile: `project.release_lines` holds the branch patterns for the main line, release lines and hotfix lines.

**P7: specs (PRD and TRD).**

Writing a PRD and a TRD is not the hard part. The hard part is that the work drifts from them while nobody notices. So the design is one source of truth plus drift detection.

- **Location: the shared knowledge repo**, because a large feature spans backend, web and mobile and one spec must be readable from every repo:
  ```
  specs/<release>/<feature>/
    prd.md   problem, users, goals, scope, Not Doing, acceptance criteria AC-1..AC-n
    trd.md   impact per repo and platform, API contracts C-1.., data and migrations,
             rollout order, risks, decisions D-1.., open questions with owners
  ```
- **Workflow `spec`** (`dashcrew spec new <feature> --release <x>` scaffolds it):
  1. Brainstorm: problem, users, options, assumptions, Not Doing. All questions are asked in one round.
  2. PRD draft with the `acceptance-criteria` skill.
  3. TRD draft with `feature-architecture`; platform packs add their sections (for example state and routing for a mobile platform, endpoints and migrations for a backend).
  4. Team review as a PR on the knowledge repo. Status goes `draft -> review -> approved`; approved means the PR merged. There is no separate approval gate.
- **Keeping the team in step:**
  - **Stable IDs:** AC-n, D-n and C-n are never renumbered.
  - **Tasks point at the spec:** step 0 gains `Spec: specs/<release>/<feature>@rev<n>`, and the task's success criteria are taken from the PRD's ACs.
  - **Drift must be declared:** when an implementation departs from the TRD, the plan states "departs from D-2: ...". `reviewer` checks that every AC is covered and that no departure is silent. An approved departure becomes a revision PR to the TRD.
  - **Revisions are tracked:** each spec carries `rev`. `doctor` warns when a plan relies on an older revision than the current one, so a member mid-task learns that the spec changed.
  - **The retro closes the loop:** after release, the retro (P5) distills the PRD, TRD and code into `features/<slug>.md`. The spec stays as the decision record; the feature page is the living knowledge.

**P8: bridge to other team tools.**
- dashcrew keeps its own formats. A bridge is a translator, with no server and no long-running process.
- An open, versioned contract `docs/interop.md` covers the knowledge page, the inbox proposal, the verified marker and a task status JSON from `dashcrew status --json` (branch, tier, chain step, last verify, PR).
- Adapters in `internal/bridge/<tool>/`: `dashcrew bridge list`, and `dashcrew bridge <tool> import knowledge | export knowledge | status`.
  - `import` writes only into dashcrew's own formats.
  - `export` writes only where the user points it, and never overwrites hand-written files without `--force`.
  - Files owned by another tool (its AGENTS.md, hooks, track files) are read only.
- Coexistence in one repo: `doctor` warns when another tool's agent instructions contradict dashcrew's, and both tools' guard hooks run as a chain, never replacing each other.
- MCP stays agent-facing: `knowledge serve` can also index knowledge imported through a bridge.
- If another tool already does workspaces and multi-repo tracks and its author agrees to split the work, P9 becomes an adapter instead of a dashcrew feature.

**P11: tracker as the team view.**
- The ClickUp pack already moves a task to `in progress` when its branch is created, and to `in review` with the PR link when the PR exists.
- P11 checks that flow live, makes it an explicit chain step in the P4 chain, and adds a GitHub Issues/Projects tracker pack for teams without ClickUp.
- It shares status only, never transcripts.
- It still runs only when the work started from a tracker task.

**P9: multi-repo tracks in dashcrew.**
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

**P10: the second platform, whatever stack the team adopts.**

P4 S0 makes core platform-agnostic. Flutter is the built-in reference platform; any stack enters through three levels:

1. **`platform: other`:** the team declares the commands in the profile and gets all of core on day one.
2. **Project platform:** the team writes `.ai/platforms/<name>/` in its own repo from its real code.
3. **Built-in:** a project platform that proves generic moves into this repo.

P10 takes the team's next stack through levels 1 and 2. Go is the current candidate; the same path applies if the team moves to another stack.

What a platform at level 2 should cover, shown for Go as the worked example:

- `platform.yaml`:
  - `detect: [go.mod]`;
  - `commands`: `go vet ./...`; `go test ./...` and its scoped form `go test ./<pkg>/... -run <Test>`; `go mod download`;
  - the linter only if the repo already configures one;
  - `link`: `go.work`, the counterpart of `pubspec_overrides.yaml`, so P9 tracks join repos.
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
- [ ] The core repo split will happen. Get a date before building P9.
- [ ] A role-trimmed skill set does not miss skills a task needed. Compare the step 0 `Skills:` lines of ten tasks with and without the profile.
- [ ] Codex and Antigravity load skills through symlinked folders. Check each live before the symlink layer ships.
- [ ] Level 1 is useful on its own. Run one real task on a non-Flutter repo with only `platform: other`, and list which missing rules the reviewer had to catch by hand. That list is the first content of the level 2 platform.
- [ ] The P10 platform's conventions match the team's repos. Read two real repos of that stack before writing a rule.

## MVP Scope

- **In P5:**
  - knowledge repo layout and profile key `knowledge: { repo, path }`;
  - retro proposals to `inbox/`;
  - `knowledge review`, `knowledge doctor` and `knowledge serve` (MCP, three tools);
  - the generated index README;
  - the AGENTS.md rule.
- **In P11:** live check of the ClickUp flow, the flow as a chain step, a GitHub tracker pack.
- **In P9:** workspace, tracks, overrides, merge-order check, one-repo track replacing `worktree`.

## Not Doing (and Why)

- **A hosted website or monitor dashboard:** the tracker board already shows who is on what; a server adds auth, hosting and data to protect.
- **A database as the source of truth:** Markdown in git is reviewable, diffable and synced by the tools the team already uses. Any index is derived and rebuildable.
- **Fully automatic knowledge writes:** unreviewed knowledge goes stale and an MCP answering confidently from it is worse than no answer.
- **Embeddings in the MVP:** only if BM25 misses too often on real questions.
- **Sharing agent transcripts:** status yes, transcripts no.
- **Moving rules into the knowledge repo:** rules stay in AGENTS.md and project packs. Knowledge holds facts and flows. A retro proposal that is really a rule is routed to the project pack instead.

## Open Questions

- Knowledge repo access: one repo for the whole team, or one per product when several products share members?
- Where the local clone lives: under `~/.dashcrew/knowledge/<name>` (proposed), or next to the workspace in P9?
- How `knowledge serve` notices a pull: rebuild the index on every call when HEAD moved (proposed), or watch the folder?
- P9 base per repo: one `--from` for all repos, or per repo like `core=origin/develop,app_a=origin/main`?
- Personal profile and symlink layer: their own phase after P5, or folded into P5? Knowledge linking needs the symlink layer, and role skills need stack packs beyond the reference stack.
- `me.yaml` per repo override (for example, `lead` in one repo, `fe` in another): needed, or one role per machine is enough?
