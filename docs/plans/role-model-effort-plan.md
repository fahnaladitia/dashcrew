# Profile overrides for the model and effort of each role

Tier: standard (profile, adapters, docs; generated output changes only for a profile that sets the new key).

## Problem

The `reviewer` and `explorer` subagents take their model and effort from `core/roles/<name>/role.yaml` (reviewer `high`/`high`, explorer `standard`/`low`). A team cannot change them: a reviewer on Opus with high effort is costly for a small project, and a team whose plans and merge decisions rest on what the explorer maps and the reviewer finds may want the explorer above `low` effort. Today the only way is a fork of the catalog.

## Profile

A new top-level key in `.ai/project.yaml`, `roles`, a map of role name to overrides:

```yaml
roles:
  reviewer: { model: standard, effort: medium }
  explorer: { effort: high }
```

- `model`: `high`, `standard` or `inherit` (the session's model). `effort`: `low`, `medium`, `high` or `inherit` (the agent's default).
- A key left out keeps the `role.yaml` value; a profile without `roles` renders byte for byte as today.
- Errors, in the profile's style: `roles: expected a map`, `roles.<name>: unknown role, expected one of explorer, reviewer`, `roles.<name>.<key>: unknown key`, `roles.<name>.model: expected high, standard or inherit`, `roles.<name>.effort: expected low, medium, high or inherit`.
- Role names: `profile.RoleNames = []string{"explorer", "reviewer"}`, the parser's list. Roles live only in `core/roles/` (packs and platforms never add one), so a catalog test asserts the list equals the embedded catalog's roles; a new role fails that test until the list is updated.
- `Profile.Roles map[string]RoleOverride` (`Model`, `Effort *string`), kept out of `ToContext`: templates never see it.

## Adapters

`rolesIn` (`internal/adapters/adapter.go`) takes the profile and yields each `RoleMeta` with the overrides applied, so the three adapters stay as they are apart from `inherit`:

| | `model` | `effort` |
|---|---|---|
| Claude Code (`.claude/agents/<role>.md`) | `high` → `opus`, `standard` → `sonnet`, `inherit` → no `model` line | the value; `inherit` → no `effort` line |
| Codex (`.codex/agents/<role>.toml`) | not written, as today (Codex has no tier names to map) | `model_reasoning_effort`; `inherit` → no line |
| Antigravity (`.agents/agents/<role>.md`) | `model: inherit`, as today | no field, as today |

`inherit` drops the line instead of writing `model: inherit`: Claude Code resolves a subagent's model as the spawn parameter, then frontmatter `model`, then `CLAUDE_CODE_SUBAGENT_MODEL`, then the parent's model, so a written `model: inherit` would shut out a member's `CLAUDE_CODE_SUBAGENT_MODEL`. With no line, a member who sets nothing still gets the parent's model, the same as `inherit`. In the same way, no `effort` line lets `CLAUDE_CODE_SUBAGENT_EFFORT` (Claude Code) and `[agents] default_subagent_reasoning_effort` (Codex) fill it. `inherit` is therefore how a team leaves a role's model or effort to each member.

The guard, `usage` and doctor read only `Capability`, which the profile cannot change.

## Tests first

- `internal/profile/profile_test.go`: a profile with `roles` parses to the overrides; each error above; no `roles` gives an empty map.
- `internal/catalog` (or `internal/adapters`): `profile.RoleNames` equals the sorted names of the embedded catalog's roles.
- `internal/adapters`: with `roles: { reviewer: { model: standard, effort: medium }, explorer: { model: inherit, effort: inherit } }`, Claude Code writes `model: sonnet` / `effort: medium` for the reviewer and neither a `model` nor an `effort` line for the explorer; Codex writes `model_reasoning_effort = "medium"` and no effort line; Antigravity is unchanged.

## Docs

- `docs/profile.md`: a `roles.<name>.model`, `roles.<name>.effort` row, with what each agent does with it (the table above, short).
- `docs/team.md`: what a member can set on their own machine once the team writes `inherit`, and what it costs (see Not doing): Claude Code `CLAUDE_CODE_SUBAGENT_MODEL` and `CLAUDE_CODE_SUBAGENT_EFFORT` in `~/.claude/settings.json` `env`; Codex `[agents] default_subagent_model` and `default_subagent_reasoning_effort` in `~/.codex/config.toml`; Antigravity `/model` and `/effort`. Each line names the agent version it needs, checked against the agent's docs when written.
- `docs/catalog-format.md`, `role.yaml`: a profile can override `model` and `effort`, not `capability`.
- `CHANGELOG.md`, `## Unreleased`: one bullet.

## Goldens

None expected: no golden profile sets `roles`. Running `DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard` must leave `git diff --stat test/golden cmd/dashcrew/testdata internal/guard/testdata` empty.

## Verify

`gofmt -l .`, `go vet ./...`, `go test ./...`; `dashcrew sync` in a scratch project with the `roles` example, then read the three agents' role files.

## Not doing

- Writing a member's choice into the role files. They are committed and recorded in `.ai/lock.json`: a personal value would make `sync` differ between members, fail `dashcrew check` in CI and rewrite the files on every PR. A user-level agent of the same name (`~/.claude/agents/`, `~/.codex/agents/`, `~/.gemini/config/agents/`) does not help either: every agent lets the project's file win a name clash.
- Overriding a value the team did write, on a member's machine. Claude Code has no per-agent setting for it: `CLAUDE_CODE_SUBAGENT_MODEL` only fills a missing `model` (since 2.1.251); beating a written one takes `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`, which forces one model on every subagent, and `CLAUDE_CODE_EFFORT_LEVEL` raises effort for the main session too. Codex applies an agent file's `model_reasoning_effort` over every user setting, and `[agents.<name>]` accepts no model or effort key. docs/team.md names these global switches and their reach; it does not recommend them.
- A `roles` key in `~/.dashcrew/me.yaml`, a follow-up after this change: `dashcrew me sync` would write each member's choice into per-user runtime config only (a Claude Code mod with `agent.spawn` and `turn.step` hooks for per-agent model and effort; a marked block in `$CODEX_HOME/config.toml` for `[agents] default_subagent_*`; nothing for Antigravity), never into the role files, and `doctor` would show the value in effect and where it comes from. It needs the mod's user-scope install and the Codex block tried on live CLIs first. Research notes: `../dashcrew-research/reports/Pengaturan model subagent per user.md`, outside the repo.
- A team key that stops members from overriding: dashcrew cannot enforce it on a member's machine; that is the organization's managed settings (`availableModels`, `maxEffortLevel`).
- `capability` overrides: they change what the guard allows a subagent to do.
- `xhigh` or `max` effort, or concrete model IDs: the values stay agent-neutral tiers; a later change can add one once every agent maps it.
- A Codex `model` line: Codex model names differ per provider and are left to the user's Codex config.
