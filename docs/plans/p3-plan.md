# P3 plan: Codex adapter, and top-3 packs per library family

Tier: large (new adapter, adapter interface refactor, new built-in packs in seven families, golden fixtures). P2 is done: `other`, project packs, flavors, `init`, `worktree`.

Two parts, both before P4:

- Part A: render the toolkit for the Codex CLI next to Claude Code, from the same catalog.
- Part B: every library family ships the three most used pub.dev options plus `none`, so most projects need neither `other` nor a project pack.

## Not doing (P3)

- Plugin or marketplace packaging and pub.dev publish (P4).
- Role capability enforcement under Codex: its `PreToolUse` input carries no subagent identity, so the guard cannot tell a read-only role from the main session. Under Codex, role limits are instructions only; the guard still enforces git, formatter and import rules.
- Codex skill metadata (`agents/openai.yaml`), icons, Windows paths.
- New options for `tracker`, `qa`, `design`: they are not pub.dev libraries.

## Part A: Codex adapter

Sources (official docs, read 2026-10-31): hooks, skills, subagents, MCP pages of the Codex documentation.

| Codex surface | Location | Format | Notes |
|---|---|---|---|
| Rules | `AGENTS.md` | shared with Claude Code | one file for both agents |
| Skills and workflows | `.agents/skills/<name>/SKILL.md` | front matter `name`, `description` | skill list budget: 2% of context or 8,000 characters |
| Roles | `.codex/agents/<name>.toml` | `name`, `description`, `developer_instructions`, optional `model_reasoning_effort` | no tool restriction field |
| Guard | `.codex/hooks.json` | `PreToolUse` with `matcher`, `type: command`, `command`, `timeout` | shell arrives as `Bash`, edits as `apply_patch` with the patch in `tool_input.command`; exit 2 + stderr blocks |
| MCP | `.codex/config.toml` | `[mcp_servers.<name>]` `command`, `args` | project `.codex/` is loaded only for a trusted project |

Trust: Codex ignores the project `.codex/` layer until the project is trusted, and runs a hook only after its hash is approved in `/hooks`. dashcrew cannot grant either; `doctor` reports it.

### A0: spike against the installed Codex CLI

Before writing the adapter, confirm what the docs leave open, in a scratch repo:

- a hyphenated role name (`flutter-planner`) in `.codex/agents/` is accepted;
- the exact `tool_input` of `Bash` (string or argv) and of `apply_patch` (one patch, several files);
- `.agents/skills` is read from the repo root and from a worktree;
- the AGENTS.md size Codex reads by default (expected 32 KiB, `project_doc_max_bytes`).

This needs a few real Codex turns with the maintainer's own Codex login.

### A1: adapter interface

- `Adapter` with `files(profile, targets, context, warnings)` for generated files and `merge(projectDir, lock, warnings)` for files shared with the user (settings, MCP, hooks).
- `ClaudeCodeAdapter` moves its settings and `.mcp.json` merges out of `Sync`. `CLAUDE.md` and `.claude/` are written only when `claude-code` is selected.
- Lock: `mcp_servers` becomes per agent (`{"claude-code": [...], "codex": [...]}`); a schema-1 lock without it is read as Claude Code's.
- No output change: both goldens stay byte-equal except the lock.

### A2: skills and workflows

- `.agents/skills/<name>/SKILL.md` for every skill and workflow, body identical to the Claude Code one.
- Front matter has `name` and `description` only; `when_to_use` is appended to the description (`<description> Use when: <when_to_use>`), the argument hint becomes the first body line.
- Test and doctor check `codex-skills-budget`: the summed front matter of the rendered skills stays under 8,000 characters for the sample stacks; doctor warns above it.

### A3: roles

- `.codex/agents/<role>.toml`: `name`, `description`, `developer_instructions` (the role body as a TOML multi-line basic string, escaped), `model_reasoning_effort` from `role.yaml` `effort`. No `model`: it inherits the session's.
- A `read` or `write-plans` role gets one extra first line in its instructions stating the limit, since Codex cannot enforce it.
- TOML is written by dashcrew itself (fixed shape, string escaping tested); no TOML dependency.

### A4: guard under Codex

- `.codex/hooks.json` `PreToolUse`, matcher `^(Bash|apply_patch|Edit|Write)$`, command `"$(git rev-parse --show-toplevel)/.ai/bin/guard" "$(git rev-parse --show-toplevel)/.ai/guard.json"`, merged like Claude Code's settings (user hooks kept, exactly one dashcrew entry).
- `GuardEvent.fromHook`: an `apply_patch` touching several files is evaluated per file (`*** Add File`, `*** Update File`, `*** Delete File`, `*** Move to`), and the forbidden-import check reads only added lines. One blocked file blocks the patch.
- Tests: patch with two files where the second imports a forbidden package; `Bash` input as string and as argv.

### A5: MCP

- `.codex/config.toml` gets a block between `# dashcrew:begin` and `# dashcrew:end` holding `[mcp_servers.dart-mcp-server]`. Everything outside the block is the user's and is never rewritten.
- The same server table already defined outside the block: warn and skip, like `.mcp.json`.

### A6: doctor, worktree, gitignore

- Doctor runs the checks of every selected agent: `codex-hook` (dashcrew entry present in `.codex/hooks.json`), `codex-mcp`, `codex-skills-budget`, `codex-trust` (warn when `$CODEX_HOME/config.toml`, default `~/.codex/config.toml`, does not mark the project trusted; the hook hash approval in `/hooks` is named in the detail).
- `dashcrew worktree` links `.codex` and `.agents` too when the main checkout ignores them.

### A7: agent-neutral wording

- AGENTS.md is shared, so no text may assume one agent. Workflow references such as `/plan` become "the `plan` workflow" (about 50 places in `core/` and `packs/`). Claude Code still gets slash commands, because its skills keep their names.
- Remaining agent-specific words in `core/` (background monitor, message tool) are rephrased or moved to the adapter.

### A8: golden and dry run

- Golden `sample-codex`: synthetic profile with `agents: [claude-code, codex]`.
- Dry run on a private sample project: sync, doctor, then `codex exec` with the project trusted: a rule question answered from AGENTS.md, and a `git push` blocked by the guard.

## Part B: top-3 packs per library family

Rule: per family, the three packages with the most pub.dev downloads in the last 30 days whose own documentation places them in that family, plus `none`. Anything else stays `use: other`. Numbers read 2026-10-31 from the pub.dev API (`/api/packages/<name>/score`).

| Family | Options (30-day downloads) | Existing | New packs | Left to `other` |
|---|---|---|---|---|
| state | riverpod (flutter_riverpod 4.15M), bloc (flutter_bloc 2.28M), provider (1.17M) | bloc | riverpod, provider, none | get (0.76M), mobx |
| di | riverpod (4.28M), get_it (2.45M), provider (1.17M) | get_it | riverpod, provider, none | injectable (0.41M, a code generator over get_it: a `get_it` param instead) |
| router | go_router (5.30M), getx (get 0.76M), auto_route (0.41M) | getx | go_router, auto_route, none | beamer |
| network | http (14.40M), dio (5.35M), retrofit (0.65M) | dio | http, retrofit, none | chopper, graphql_flutter |
| storage | shared_preferences (8.97M), sqflite (3.25M), hive (hive_ce 1.41M + hive 1.20M) | hive, none | shared_preferences, sqflite | drift (1.84M); flutter_secure_storage (5.84M) is the security rule's concern |
| test | mocktail (3.90M), mockito (2.76M), manual-fakes (no package) | manual-fakes | mocktail, mockito, none | bloc_test (a bloc helper, goes into the bloc pack) |
| analytics | firebase_analytics (2.59M), posthog (0.45M), mixpanel (0.26M) | posthog, none | firebase, mixpanel | sentry_flutter (error monitoring), appsflyer_sdk (attribution) |

### B1: pack content

- Each new pack: `pack.yaml` (description, params with neutral defaults), an `agents_md` fragment (placement, usage, what never to do), and fragments for workflows or roles only where the library has a trap a reviewer or tester must know (code generation step, test override, lifecycle).
- State packs ship a patterns skill like `bloc-patterns` (`riverpod-patterns`, `provider-patterns`).
- Every rule is checked against the package's official documentation; the plan for each pack lists the pages used.
- A `none` pack is one `agents_md` line: no library for this concern, do not add one without asking.
- `di: riverpod` and `di: provider` render a short note when `state` is not the same library; no other cross-pack fragments.

### B2: init detection

- One dependency may imply several families (`flutter_riverpod` -> state and di, `provider` -> state and di, `get` -> router).
- Table rows for the new options replace their `other` rows; tests per row as in P2.

### B3: size and budget

- AGENTS.md stays under 32 KiB for every option alone and for the largest option of every family together.
- The Codex skill list stays under its budget with every state pack (A2).

### B4: golden

- Golden `sample-riverpod-gorouter`: synthetic profile with riverpod (state and di), go_router, http, shared_preferences, mocktail, firebase.

## Order and gates

Proposed: A0 -> A1 -> ... -> A8, then B1 -> B4. A7 fixes the neutral wording before Part B writes new text, and A2 measures the skill budget that Part B's new skills must fit. Each stage: tests first, `dart analyze --fatal-infos` + `dart test` on the minimum SDK and stable, golden diff reviewed, commit by the maintainer.

## Decisions needed

1. Order: Part A then Part B (proposed), or Part B first?
2. The option table in Part B, especially: `di` = riverpod, get_it, provider (injectable as a get_it param); `storage` keeps hive instead of drift and leaves secure storage to the security rule; `test` counts manual-fakes as one of three; analytics excludes error monitoring and attribution SDKs.
3. `tracker`, `qa` and `design` stay as they are (proposed)?
4. A0 and A8 run a handful of real Codex turns on the maintainer's Codex login: allowed?

## Decisions (confirmed)

1. Part A then Part B.
2. The Part B option table as written.
3. `tracker`, `qa` and `design` stay as they are.
4. A0 and A8 may run a few real Codex turns on the maintainer's login.

## Progress

- A0 waits: the Codex CLI is not installed yet on the maintainer's machine.
- A1 done. `Adapter` (`files`, `merge`) and `adapterFor` in `lib/src/adapters/adapter.dart`; `ClaudeCodeAdapter` owns `CLAUDE.md`, `.claude/settings.json` and `.mcp.json`; `ProjectFiles` holds the shared write-if-changed. Lock `mcp_servers` is keyed by agent. Doctor's agent checks move in A6.
