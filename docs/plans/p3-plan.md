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
- A2 done. `CodexAdapter` writes `.agents/skills/<name>/SKILL.md` (front matter `name`, `description`; `Use when:` appended; `Arguments:` as the first body line). Skill and workflow selection is shared with Claude Code through `skillDocs`. The full sample stack uses 4,779 of the 8,000-character skill list budget (26 skills). `adapterFor('codex')` stays refused until the guard (A4) and MCP (A5) are in, so `sync` never writes a Codex toolkit without its guard.
- A4 done. `GuardEvent.allFromHook` splits a Codex `apply_patch` into one event per file (Add -> write, Update/Delete -> edit, `Move to` checked at the new path, content = added lines); `evaluateAll` blocks on the first blocked event, and `bin/guard.dart` uses both. `CodexAdapter.merge` keeps exactly one dashcrew `PreToolUse` entry in `.codex/hooks.json` (matcher `^(Bash|apply_patch|Edit|Write)$`, timeout 30) next to the user's hooks. Checked with a compiled guard: Codex-shaped `git push` and a forbidden import in the second file of a patch exit 2; removing that import exits 0. A3 waits for A0 (hyphenated role names).
- A5 done. `.codex/config.toml` gets a block between `# dashcrew:begin` and `# dashcrew:end` with `[mcp_servers.dart-mcp-server]`, appended at the end of the file (TOML wants top-level keys before the first table) and replaced in place later; text outside the block is never rewritten. A server the user defines outside the block (table header, dotted key or inline table) is left alone with a warning. `adapterFor('codex')` now waits for A3 instead: the `orchestrate` workflow names roles, and Codex has none until A3.
- A6 done. Doctor runs the agent checks of the profile's agents (Claude Code alone when the profile cannot be read): `codex-hook` (fail without the exact dashcrew entry), `codex-mcp` (warn when the server is missing; ok when the user defines it outside the block), `codex-skills-budget` (warn over 8,000 characters), `codex-trust` (warn unless `$CODEX_HOME/config.toml`, default `~/.codex/config.toml`, has `trust_level = "trusted"` for the project or a parent). `dashcrew worktree` links `.codex` and `.agents` when the main checkout ignores them.
- A7 done. Workflow references on 36 lines in 27 catalog files lose their `/` (`# /plan` -> `# plan`, `` `/verify full` `` -> `` `verify full` ``, bare names in prose get backticks, code blocks drop the prefix). AGENTS.md gets one line (`workflow.names-without-prefix`) saying workflows are named without the agent's invocation prefix, `/plan` or `$plan`. No other Claude Code-only wording was found in `core/` or `packs/` ("subagent" and "background" exist in both agents).
- B1 state done: `state/riverpod` (param `codegen`, default false; skill `riverpod-patterns`; tdd fragment with `ProviderContainer.test`), `state/provider` (skill `provider-patterns`; tdd fragment testing the `ChangeNotifier` directly), `state/none` (Flutter only). Sources: riverpod.dev getting started, "What's new in Riverpod 3.0", refs and auto-dispose pages; the provider README on pub.dev (it has no testing guidance, so the provider test rule stays with the ChangeNotifier API). `init` maps `flutter_riverpod`, `hooks_riverpod`, `riverpod` to `state: riverpod` and `provider` to `state: provider`. Not used: the claim that `@riverpod` auto-disposes by default (third-party only).
- B1 rest, B2, B3 done in one round (maintainer asked for every family at once):
  - di: `riverpod` (providers as DI, overrides; a note for other state libraries), `provider`, `none`; `get_it` gains param `injectable` (annotations and generated config instead of hand registration).
  - router: `go_router`, `auto_route`, `none`. network: `http` (tdd fragment with `MockClient`), `retrofit`, `none`. storage: `shared_preferences`, `sqflite` (tdd fragment with `sqflite_common_ffi`). test: `mocktail` (blocTest fragment when state is bloc), `mockito`, `none`. analytics: `firebase`, `mixpanel` (same decision and facade rules as posthog, plus vendor lines).
  - Sources: pub.dev READMEs of get_it, injectable, go_router (and its Navigation topic), auto_route, http, retrofit, shared_preferences, sqflite, sqflite_common_ffi, mocktail, mockito, bloc_test, mixpanel_flutter; Riverpod overrides and DO/DON'T pages; Firebase's Flutter get-started page. Not used because no official page confirmed them: Firebase event name limits and FirebaseAnalyticsObserver, testing advice for go_router, auto_route and shared_preferences.
  - `init`: a dependency may fill several families (riverpod and provider fill state and di) and carry params (`injectable` -> `di: { use: get_it, injectable: true }`); every new option replaces its `other` row.
  - B3: the largest option of every family together renders 22,470 of 32,768 bytes of AGENTS.md (test added). No new skills besides the two state ones, so the Codex skill budget is unchanged.
- B4 done. Golden `sample-riverpod-gorouter`: riverpod with `codegen: true` for state and di, go_router, http, shared_preferences, mocktail, firebase, semantics identifiers, one flavor. Sync output, idempotent second sync, and doctor without failures besides the uncompiled guard. Part B is complete.
- A0 done without model turns (the maintainer has no Codex subscription), against Codex CLI 0.166.0: `codex debug prompt-input` in a scratch repo, and the `rust-v0.166.0` source.
  - AGENTS.md is cut at 32 KiB (`DEFAULT_PROJECT_DOC_MAX_BYTES = 32 * 1024`; a codeword at byte 31,102 is in the prompt, one at 33,582 is not), so dashcrew's limit matches.
  - `.agents/skills/<name>/SKILL.md` at the repo root is listed with its description.
  - A role file's `name` only has to be non-empty after trimming; the character limit applies to `nickname_candidates`. `flutter-planner` is valid. A role discovered from a folder must have `developer_instructions`.
  - `PreToolUse` input: shell runs as `tool_name: "Bash"` with `tool_input: {"command": "<string>"}`; edits as `tool_name: "apply_patch"` (matcher aliases `Write`, `Edit`) with `tool_input: {"command": "<patch text>"}`.
  - Unlike the docs page, `PreToolUse` input carries `agent_id` and `agent_type` for a spawned subagent; `agent_type` is the role name it was spawned with (`default` without one) and is absent in the main session. The guard already reads `agent_type`, so role capabilities are enforced under Codex too.
- Plan changes from A0: the "Not doing" item about role enforcement under Codex is void, and A3 drops the extra instruction line it planned for that. A8 keeps the `sample-codex` golden but replaces the `codex exec` dry run with `codex debug prompt-input` on a synced sample plus a compiled guard fed Codex-shaped input, since no model turn is available.
- A3 done. `CodexAdapter` writes `.codex/agents/<role>.toml` (`name`, `description`, `model_reasoning_effort` from `effort`, `developer_instructions` as an escaped multi-line basic string) for every generated role, with the same `when` gating as Claude Code. `adapterFor('codex')` is enabled; `agents: [codex]` writes no Claude Code files, and both agents together share AGENTS.md and the guard policy. Checked on a synced sample: the 13 role files parse with Python's `tomllib`, satisfy the 0.166.0 loader rules, and their parsed bodies equal the Claude Code role bodies; `codex debug prompt-input` shows AGENTS.md and the skills; the compiled guard blocks `apply_patch` from `flutter-code-reviewer` (`agent_type`), allows it from `flutter-engineer`, and blocks `git push`.
- A8 done. Golden `sample-codex` (claude-code and codex; provider, auto_route, retrofit, sqflite, mockito, mixpanel with an EU server URL) passes the golden checks, including doctor with the Codex checks. `test/golden/codex_toml_test.dart` parses its `.codex/agents/*.toml` and `.codex/config.toml` with Python's `tomllib` and compares each role body with the Claude Code one; it skips without Python 3.11+. A one-character change in a golden role body turns it red. `codex debug prompt-input` and the compiled guard with Codex-shaped input were checked by hand in A3; neither runs in CI, which has no Codex CLI.
- P3 is complete. Not covered: a real Codex model turn (no subscription available).
