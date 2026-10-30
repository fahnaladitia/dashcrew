# P1 stage 5: guard

Tier: standard. Library + entry point + tests. Wiring into agent hook files is stage 6.

## Shape

- `GuardPolicy` is compiled from profile + catalog by `sync` and written to `.ai/guard.json`. The guard binary reads only that file and the hook event, so startup stays at AOT speed (no YAML, no catalog walk).
- `GuardEvent.fromHook(json)` normalizes Claude Code and Codex hook input: tool kind (shell, write, edit), command text, file path, written content, role (`agent_type`).
- `evaluate(event, policy)` returns allow or block with a reason. The binary exits 2 with the reason on stderr when blocked (both agents treat exit 2 as block), 0 otherwise. Unreadable input or policy never blocks (exit 0) so a broken install cannot lock the user out; `doctor` reports it instead.

## Shell analysis

Commands are found by a small tokenizer, not a regex over the raw text:

- Separators: `;` `&` `&&` `||` `|` newline, `(` `)`.
- Quoted text ('...' and "...") is data. Command substitutions `$(...)` and backticks are commands, also inside double quotes.
- Heredoc bodies (`<<EOF`, `<<-EOF`, `<<'EOF'`) are data.
- Leading `VAR=value` assignments and wrappers (`sudo`, `env`, `command`, `nohup`, `time`, `rtk`) are skipped.
- `git` global options (`-C <dir>`, `-c <k=v>`, `--git-dir=...`, `--work-tree=...`, `--no-pager`, `-P`) are skipped before the subcommand.

## Policy (schema 1)

| Field | From |
|---|---|
| `blocked_git` | always `push stash reset rebase cherry-pick revert merge tag`; plus `commit` when `policy.agent_commit` is false |
| `block_formatters` | `!policy.formatter`: `dart format`, `dart fix`, `flutter format` |
| `plans_dir` | `paths.plans` |
| `role_capability` | role name -> `read` / `write-plans` / `write-code` from `core/roles/*/role.yaml` |
| `forbidden_imports` | analytics pack `vendor_import` when set: blocked in app `lib/` outside `project.local_packages` |

Write rules: `read` roles cannot write or edit; `write-plans` roles may only `write` under `plans_dir` and never `edit`.
