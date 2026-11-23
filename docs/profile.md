# Profile, platforms and skills

The reference for `.ai/project.yaml`, the platforms a project can pick, the stack options of each, and how agents choose skills.

## The profile: `.ai/project.yaml`

```yaml
toolkit: 1                                # required
platform: flutter                         # flutter (default), a project platform, or other
agents: [claude-code, codex, antigravity] # required: every agent your team uses
language: { dartdoc: en, pr: en, chat: en }
stack:
  state: { use: riverpod, codegen: true } # a pack, with params
  di: riverpod
  router: go_router
  network: { use: other, name: graphql_flutter }   # no pack: generic rules for that library
project:
  flavors: { dev: lib/main_dev.dart, prod: lib/main_prod.dart }
  local_packages: [packages/core]
  base_branches: [main, develop]
  branch_prefix: { feat: feat/, fix: fix/, hotfix: hotfix/ }
  build_wrapper: tool/build.sh
policy:
  agent_commit: false
  agent_push: false
  formatter: false
  dartdoc_max_lines: 3
  tier_gates: true
  terse_prompts: false
paths: { plans: .ai/plans, audits: .ai/audits, pr: .ai/pr, memory: .ai/memory }
ci: { github: true, dashcrew_ref: v0.1.0 }
```

| Key | Default | Effect |
|---|---|---|
| `platform` | `flutter` | `flutter`, a project platform in `.ai/platforms/<name>/`, or `other` with `platform_config`; see [Platforms](#platforms) |
| `agents` | none, required | `claude-code`, `codex`, `antigravity`; files are written for each |
| `language.dartdoc`, `.pr`, `.chat` | `en` | language of Dartdoc, PR descriptions and chat (ISO 639-1) |
| `stack.<family>` | not set | an option name, or `{use: <option>, <param>: ...}`; see [Stack options](#stack-options) |
| `project.flavors` | none | flavor name to entrypoint; rendered as `flutter run --flavor <name> -t <entrypoint>` |
| `project.local_packages` | none | path packages; the app's `lib/` rules and the vendor import check skip them |
| `project.base_branches` | `[main]` | proposed bases for new branches |
| `project.branch_prefix` | `feat/`, `fix/`, `hotfix/` | branch prefixes per change type |
| `project.build_wrapper` | none | script that injects `--dart-define`s for real builds |
| `project.commit.pattern` | `<type>(<scope>): <subject>` | commit message pattern; placeholders `type`, `scope`, `subject` (required), `ticket`. An empty scope or ticket is dropped with its brackets |
| `project.commit.types` | `feat, fix, refactor, test, docs, chore, perf, build, ci` | allowed types |
| `project.commit.type_from_branch` | `{feat: feat, fix: fix, hotfix: fix}` | `branch_prefix` key to type |
| `project.commit.scopes` | none | path glob to scope, first match wins (`lib/features/cart/**: cart`); the scope with the most changed files is picked |
| `project.commit.ticket` | none | `{from_branch: <regex>, trailer: <name>}`: ticket id from the branch name (first group, else the match); `trailer` defaults to `Refs` unless the pattern has `<ticket>` |
| `project.commit.subject_max` | `72` | longest subject line, in characters |
| `project.commit.language` | `en` | language of commit subjects and bodies |
| `project.commit.body` | `optional` | `optional`, `required` or `none` |
| `project.commit.trailers` | none | trailers allowed on top of `Co-authored-by`, `Signed-off-by`, `Reviewed-by` and the ticket trailer |
| `project.commit.allow_email_trailers` | none | the only trailers whose value may hold an email address (DCO `Signed-off-by`); an email anywhere else fails lint |
| `project.release_lines` | not set | `{main: <branch>, release: <regex>, hotfix: <regex>, check_ports: <bool>}`: the main line and the patterns of release and hotfix lines; see [Release lines](release-lines.md) |
| `project.links` | not set | `[{package: <name>, repo: <git url>}]`: dependencies a task may point at a local checkout with `dashcrew link` (the pubspec name or Go module path; `repo` optional, ssh or https). `has_links` gates the multi-repo text |
| `policy.agent_commit` | `false` | `true`: at the end of each task the agent asks whether to commit for you or leave it to you, and commits only on your answer |
| `policy.agent_push` | `false` | `true` (needs `agent_commit: true`): the same question covers the push of the task branch. The guard still blocks a force push, a deleted remote branch, `--all`, `--mirror`, `--tags`, a push to a `project.base_branches` branch, and a push that does not name its branch. Needs dashcrew 0.7.0 on every machine |
| `policy.agent_commit: ask`, `agent_push: ask` | | the agent runs `git commit` (and, with `agent_push: ask`, the push of the task branch) without the question round, and the guard has the agent ask you to approve each call: a Claude Code permission prompt, an Antigravity `force_ask`. The question is enforced, also for `bash -c "git commit"` and the other forms the guard unwraps. Codex hooks cannot ask, so Codex calls are blocked and its agent prints the commands. `agent_push: ask` needs `agent_commit: ask`. Needs dashcrew 0.8.0 on every machine, and a `.claude/settings.json` written by dashcrew 0.8.0 or newer |
| `policy.formatter` | `false` | `true` allows `dart format` on changed files only. Either way, verify runs the read-only check `dart format --output=none --set-exit-if-changed` on the changed files; with `false` the agent fixes what it lists by hand |
| `policy.dartdoc_max_lines` | `3` | hard cap per documented unit |
| `policy.tier_gates` | `true` | trivial and small tasks with a GO plan continue without waiting |
| `policy.terse_prompts` | `false` | agents answer short and blunt |
| `paths.*` | `.ai/plans`, `.ai/audits`, `.ai/pr`, `.ai/memory` | where plans, audits, PR descriptions and project memory live |
| `ci.github` | `false` | write `.github/workflows/dashcrew.yml`, which runs `dashcrew check` on every pull request; see [Merge gate on GitHub](team.md#merge-gate-on-github) |
| `ci.dashcrew_ref` | none | dashcrew release tag that workflow installs, `v0.1.0` or newer; required with `ci.github`. A commit or branch is refused |
| `knowledge` | not set | `{repo: <git url or path>, name: <folder>}`: the team's shared knowledge repo; see [Shared knowledge](knowledge.md) |
| `roles.<name>.model`, `.effort` | the role's `role.yaml`: `reviewer` `high`/`high`, `explorer` `standard`/`low` | model and effort of the `reviewer` and `explorer` subagents. `model`: `high` (Claude Code `opus`), `standard` (`sonnet`) or `inherit`; `effort`: `low`, `medium`, `high` or `inherit`. `inherit` leaves the line out, so the subagent takes the session's value or each member's own default; see [Subagent model and effort](team.md#subagent-model-and-effort). Codex reads only `effort`, Antigravity neither: its subagents always run on the session's model and effort |

## Platforms

The profile's `platform` picks one. Every stack works at the first level; the others add depth.

| Level | How | What the team gets |
|---|---|---|
| `platform: other` | declare it inline: `platform_config: { detect: [<file>], commands: { analyze, test, test_scoped, deps, format }, link }` (`format` and `link` optional; `link` is `pubspec_overrides.yaml` or `go.work`) | all of core: the chain, skills, guard, `reviewer` and `explorer`, PR template and `dashcrew check`; core renders your commands, with no stack-specific rules |
| Project platform | write `.ai/platforms/<name>/` in your repo, in the format of `platforms/` (`platform.yaml`, rules, skills, workflows, packs), and set `platform: <name>` | depth for that stack, your conventions, no dashcrew release needed |
| Built-in | `platforms/<name>/` in this repo: `flutter`, `go` | shared with every team, updated with dashcrew |

```yaml
platform: other
platform_config:
  detect: [pyproject.toml]
  commands: { analyze: ruff check ., test: pytest, test_scoped: pytest, deps: pip install -e ., format: ruff format --check }
```

**Go.** `platform: go`, which `init` picks for a `go.mod`, gives a Go service: Go rules (context, error wrapping, goroutine ownership, `gofmt -l` as a check), backend architecture, security and testing rules in place of the app-oriented core ones, a `backend-security` review skill, the `data-layer-tdd` and `feature-architecture` skills for services, the `http` and `db` stack families below, `link: go.work`, and the backend `guard:` rules.

**Ejecting.** `dashcrew platform new <name> [--http ...] [--db ...]` copies a built-in platform (`flutter` or `go`) into `.ai/platforms/<name>/`, where it replaces the built-in one, so a team can rename the layers and rewrite the rules to match its code. The folder is the team's from then on: dashcrew never overwrites it, and it no longer gets the built-in platform's updates. For `go`, `--http` and `--db` default to what `go.mod` (and a `sqlc.yaml`) names. Without `.ai/project.yaml` it writes one; with one, it prints the keys to set. Smaller changes do not need it: a project pack or the AGENTS.md project block covers most.

A project platform with a built-in name replaces it whole. The format is in [docs/catalog-format.md](catalog-format.md#platforms).

## Stack options

The `flutter` platform ships these families, and `go` ships `http` (`nethttp`, `chi`, `gin`, `echo`, `fiber`, `none`) and `db` (`pgx`, `sqlc`, `gorm`, `none`); `tracker` and `design` are shared by every platform.

| Family | Options | Shipped with dashcrew |
|---|---|---|
| `state` | `bloc`, `riverpod`, `provider`, `none` | rules, a patterns skill, test guidance |
| `di` | `get_it` (param `injectable`), `riverpod`, `provider`, `none` | registration rules |
| `router` | `go_router`, `auto_route`, `getx`, `none` | navigation rules |
| `network` | `dio`, `http`, `retrofit`, `chopper`, `none` | client placement, error mapping, test client |
| `storage` | `hive`, `shared_preferences`, `sqflite`, `drift`, `none` | access through repositories, migrations |
| `test` | `manual-fakes`, `mocktail`, `mockito`, `none` | mocking conventions |
| `analytics` | `posthog`, `firebase`, `mixpanel`, `amplitude`, `none` | ask-before-tracking, facade-only imports |
| `tracker` | `clickup`, `github` (param `mode`: `project` or `labels`), `none` | task status flow: start when the branch exists, review with the PR link when the PR exists, only for work that started from a task |
| `qa` | `semantics-identifier` | QA identifier rules |
| `design` | `figma`, `none` | design-to-code skill (`figma-design`) |

Every family also takes `{use: other, name: <package>}` for a library without a pack.
Pack params and their defaults are in `pack.yaml` of each pack (`platforms/flutter/packs/<family>/<option>/` or `packs/<family>/<option>/`); set them in the map form (`state: {use: bloc, shared_dir: lib/shared/blocs/}`).

**GitHub tracker.** `tracker: {use: github, project_owner: <owner>, project_number: <n>}` keeps status in the Project's single-select `Status` field (`In Progress`, `In Review`; params `status_field`, `status_start`, `status_review`). A new GitHub Project's built-in `Status` has Todo, In Progress and Done: add an `In Review` option to it rather than a second field, since GitHub's workflows set `Status` when an item is added and a board grouped by `Status` would not show a second field's value; `mode: labels` uses issue labels instead (`label_start`, `label_review`). The PR body links the issue with `pr_link` (`Refs` by default, so merging never closes the issue unasked; set `Closes` to close it). Projects need gh 2.31 or newer and the `project` scope (`gh auth refresh -s project`); `doctor` warns with `tracker` when the `gh` on PATH has no `gh project`. The agent never creates a Project, field, option or label; a missing one stops the flow with a question. With any tracker, the `branching` and `pr-create` skills carry the flow's two steps.

**Project packs.** Put your own packs in `.ai/packs/<family>/<option>/`, in the same format. One with a shipped name replaces it; a new family (`architecture/clean`) becomes a valid `stack` key for that project. See [docs/catalog-format.md](catalog-format.md).

## Choosing skills

Skills are picked once per task, at step 0, in one visible list. The agent prints this block before any work (the `prompt-polish` skill holds the template), asks its questions with the agent's question tool when it has one, and you confirm or edit it once:

```text
STEP 0  feat: Add a status filter (all, paid, unpaid) to the order list.
========================================================================
Scope      lib/features/orders/ (order_list_page, order_list_cubit,
           order_repository), test/features/orders/
Not doing  keeping the filter across app restarts
Tier       standard  (4 files across ui, state and data)

-- Assumptions ---------------------------------------------------------
A1    the orders API accepts ?status=paid|unpaid

-- Questions -----------------------------------------------------------
Q1    default filter: "all", or the last one used?

-- Skills --------------------------------------------------------------
NAME                 WHY
bloc                 state lives in OrderListCubit
data-layer-tdd       new query param in OrderRepository

-- Success criteria ----------------------------------------------------
ID    CRITERION                                 TEST
AC-1  picking "paid" shows only paid orders     widget
AC-2  repository sends status=paid              unit

-- Branch --------------------------------------------------------------
1     feat/order-status-filter
2     feat/orders-filter-by-status
3     feat/order-list-filter
base  develop or main?
========================================================================
Reply "ok", or answer by id (Q1 ...), pick a branch and a base, edit any line.
```

That is a flutter task; other platforms say `ui` where it says `widget`. Review, verify, learn, build-fix and the end-of-task summary print in the same layout, and so does any comparison: a table aligned with spaces, so every value copies clean.

- **The index.** AGENTS.md has a generated `### Skill index`: one line per installed skill with its `when_to_use`, sorted by name. The agent writes the step 0 `Skills` list from it. Nobody keeps that table by hand: a skill in a pack or project platform appears once it renders, and `doctor` checks that every indexed skill is installed.
- **Review checklists.** A skill whose `meta.yaml` sets `review: true` is marked `(review)` in the index. When it is in the `Skills` list, the `reviewer` applies its checklist to the diff. Shipped ones: `design-fidelity`, plus `mobile-security` and `qa-identifiers` (with `qa: semantics-identifier`) on flutter.
- **Your own.** Add a skill to a project pack (`skills/<name>.yaml` plus a `target: skill/<name>` fragment) or a project platform (`skills/<name>/meta.yaml`), with a `when_to_use` that says when to pick it.
- **Project skills.** A skill the team adopts lives in `.ai/skills/<name>/`: a `SKILL.md` copied as written (never rendered, so `{{` survives), a `meta.yaml` (`description`, `when_to_use`, `review`, `optional`), and any supporting files (`references/`, `scripts/`), written next to each agent's `SKILL.md` with their executable bit kept. Hidden files are skipped; a symlink, a pipe or device, a file over 1 MiB, or a file past the first 50 is left out with a warning from `sync` and `doctor`. A required one (`optional: false`, the default) is written for every agent and joins the skill index. An optional one is never written by `sync`: a member turns it on with `skills: [<name>]` in their `.ai/me.yaml`, and `dashcrew me sync` writes it into that member's clone only, listed in `.git/info/exclude`, so a member who did not turn it on has no file any agent could pick. A `review: true` skill is the team's checklist and cannot be optional. The usual path: try a skill alone in `~/.dashcrew/skills/<name>/` with `skills: [<name>]` in `~/.dashcrew/me.yaml`, then `dashcrew skill adopt <name>` (or `<folder>`, `--required`).
