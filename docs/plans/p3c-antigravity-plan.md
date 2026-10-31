# P3c plan: Antigravity adapter

Tier: large (new adapter, new guard input and output format, profile agent value). Comes after P3 (Codex adapter, top-3 packs) and before P4 (plugin packaging, publish).

Goal: `agents: [antigravity]` renders the same toolkit for Google Antigravity, alone or next to Claude Code and Codex.

Sources (antigravity.google docs, read 2026-10-31): Rules, Skills, Hooks, MCP, Subagents.

## What Antigravity reads

| Surface | Location | Format | Notes |
|---|---|---|---|
| Rules | `AGENTS.md` (also `GEMINI.md`, `.agents/rules/*.md`) | no front matter, always on | each rule file is cut at 24,000 bytes; global plus always-on rules share a 20,000-token budget |
| Skills | `.agents/skills/<name>/SKILL.md` | `description` required, `name` optional | the same folder and format Codex reads |
| Subagents | `.agents/agents/<name>.md` | front matter `name`, `description`, `model` (`inherit`, `flash`, `pro`), `tools` allowlist; body is the system prompt | an unknown tool name may hang the subagent |
| Hooks | `.agents/hooks.json` | `PreToolUse` with `matcher` (regex on tool name) and `command`, `timeout` handlers | input `toolCall.name`, `toolCall.args`; block by printing `{"decision": "deny", "reason": ...}`; exit codes undocumented |
| MCP | `.agents/mcp_config.json` | `mcpServers` with `command`, `args` | stdio or Streamable HTTP |

Tool names that matter to the guard: `run_command` (`CommandLine`), `write_to_file` (`TargetFile`, `CodeContent`), `replace_file_content` (`TargetFile`, `ReplacementContent`), `multi_replace_file_content` (`TargetFile`, `ReplacementChunks`).

## Not doing

- The `write-plans` path limit under Antigravity: hook input has no subagent identity, and the `tools` allowlist cannot scope a path. It stays an instruction there; `read` roles are enforced by the allowlist.
- `.agents/rules/*.md`, `GEMINI.md`, global `~/.gemini/` files: AGENTS.md already carries the rules.
- Antigravity workflows: the docs move them to skills, which dashcrew already writes.
- Plugin packaging (P4).

## Stages

### C0: pin the open details

Before code, settle from the docs (and from the Antigravity CLI if the maintainer can run it):

- the exact top-level shape of `hooks.json` (the page shows hook names mapped to event keys);
- whether a hook command runs through a shell and from which directory (decides between `$(git rev-parse --show-toplevel)` and a relative path);
- the keys inside `ReplacementChunks`;
- what a non-zero exit or plain stderr does.

### C1: profile and adapter

- `antigravity` becomes a valid `agents` value and `adapterFor` entry.
- Skills and workflows: `.agents/skills/<name>/SKILL.md`, the Codex format (its front matter is valid for both). With Codex and Antigravity together, both adapters produce the same files; `sync` writes them once and fails loudly if the contents ever differ.
- Roles: `.agents/agents/<role>.md` with `name`, `description`, `model: inherit`, and `tools` from the role capability:
  - `read`: `view_file`, `list_dir`, `find_by_name`, `grep_search`, `run_command`
  - `write-plans`: the read tools plus `write_to_file`
  - `write-code`: plus `replace_file_content`, `multi_replace_file_content`
  - only documented tool names, checked by a test.

### C2: guard

- `GuardEvent.allFromHook` reads `toolCall.name` and `toolCall.args`: `run_command` -> shell, `write_to_file` -> write, the two replace tools -> edit with the replacement text as content.
- The guard answers in the caller's format: Antigravity input gets `{"decision": "deny", "reason": "[dashcrew guard] ..."}` on stdout and exit 0 (the documented path); Claude Code and Codex keep exit 2 with stderr.
- `.agents/hooks.json` holds one dashcrew `PreToolUse` entry, matcher `^(run_command|write_to_file|replace_file_content|multi_replace_file_content)$`, merged next to user hooks.

### C3: MCP

- `.agents/mcp_config.json`: `dart-mcp-server` merged like `.mcp.json`; a server the user defined differently is left with a warning. Lock `mcp_servers.antigravity`.

### C4: doctor and size

- With `antigravity` selected, the AGENTS.md limit is 24,000 bytes (Antigravity cuts each rule file there): the render test checks the largest stack against it, and `agents-md-size` warns above it. Today's largest stack renders 22,470 bytes, so the project block has about 1.5 KB left.
- Checks `antigravity-hook`, `antigravity-mcp`, `antigravity-roles` (every `tools` entry is a known tool name).

### C5: golden

- Golden `sample-antigravity`: all three agents, so the shared `.agents/skills` and the three guard outputs are covered.

## Decisions needed

1. Agent value `antigravity` (proposed).
2. AGENTS.md limit 24,000 bytes when Antigravity is selected (proposed), or keep 32 KiB and only warn?
3. C0: docs only (proposed if the Antigravity CLI cannot be run), or a check with the Antigravity CLI on your Google account?
4. Role `tools` allowlists as listed in C1?

## Decisions (confirmed)

1. Agent value `antigravity`.
2. AGENTS.md limit 24,000 bytes when Antigravity is selected.
3. C0 from the docs only.
4. Role `tools` allowlists as listed in C1.

## Progress

- C0 done from the docs. `hooks.json` is keyed by a hook name ("consumer tag") first, then the event: dashcrew owns the key `dashcrew-guard` and never touches other keys. The docs leave open whether a command runs through a shell, its working directory, the keys of `ReplacementChunks`, and the meaning of exit codes. Chosen: command `./.ai/bin/guard` with no argument, like the docs' `./scripts/lint.sh` example; the guard finds `.ai/guard.json` through `workspacePaths[0]` (Antigravity sends no `cwd`); a block is `{"decision": "deny"}` on stdout with exit 0; for `ReplacementChunks` the guard reads the chunk values whose key starts with `Replacement`. Remaining risk: if Antigravity runs hooks from another directory, the guard does not run; this cannot be checked without running Antigravity.
- C1 done. `antigravity` is a valid `agents` value. `AntigravityAdapter` writes `.agents/skills` through `agentsSkillFile`, now shared with `CodexAdapter` (same bytes), and `.agents/agents/<role>.md` with `model: inherit` and the `tools` allowlist of the role capability, every name checked against the documented tool list. `adapterFor('antigravity')` waits for C2 and C3, so `sync` never writes an Antigravity toolkit without its guard.
- C2 done. `GuardEvent.allFromHook` reads Antigravity `toolCall` input (`run_command` -> shell from `CommandLine`; `write_to_file` -> write; the replace tools -> edit with only the replacement text; any other tool is allowed). `hookResponse` answers per caller: `{"decision": "deny", "reason": ...}` on stdout with exit 0 for Antigravity, exit 2 with stderr for Claude Code and Codex, nothing for an allowed call (no `allow` decision, which could change the user's permission flow). `bin/guard.dart` falls back to `workspacePaths[0]` for the policy. `AntigravityAdapter.merge` owns the `dashcrew-guard` key of `.agents/hooks.json`. Checked with a compiled guard run from another directory: Antigravity `git push` and a vendor import get the deny JSON, `git status` prints nothing, Claude Code input still exits 2. The relative command path stays the C0 risk.
