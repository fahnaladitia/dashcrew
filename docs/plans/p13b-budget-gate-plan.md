# P13b: budget gate, the always-on context and the memory index line

Tier: standard. `doctor` (new checks), `core/skills/continuous-learning` (generated output: every sample's skill file and lock), goldens in `test/golden/` and the doctor lines in `cmd/dashcrew/testdata/transcripts/`. No profile key, no schema change.

Second of the four token-saving tasks (P13 usage, P13b this, P13c cheaper execution, P13d path-scoped Claude rules). `dashcrew usage` gives the before and after `start` number.

## Problem

`agents-md-size` measures AGENTS.md alone. What an agent loads before the first answer is more than that. expense_tracker (dashcrew 0.5.0), today:

| File | Bytes | How it loads |
|---|---|---|
| `AGENTS.md` | 35,109 (generated part ~21 KB) | `@AGENTS.md` in CLAUDE.md; Codex and Antigravity read it |
| `docs/ai/memory/MEMORY.md` | 19,891 | `@docs/ai/memory/MEMORY.md`, added to CLAUDE.md by hand; AGENTS.md tells every agent to read `{{paths.memory}}` |
| total for Claude Code | ~55,000 | `start` measured by `dashcrew usage`: 71.7k to 82.9k tokens |

The memory index has 97 lines: median 197 characters, p90 263, longest 474. The skill says "one line per memory" and "never put memory content in the index", but sets no length, so each hook grows into a summary.

## 1. `always-on` check in doctor

One check, for each selected agent, sums the project files that agent loads at session start:

- **claude-code**: `CLAUDE.md` and `CLAUDE.local.md`, with `@path` imports followed (relative to the importing file, depth 5 as in Claude Code, each file once); `.claude/rules/**/*.md` whose front matter has no `paths:`.
- **codex**: `AGENTS.md` (and `AGENTS.override.md` when present, which Codex reads instead).
- **antigravity**: `AGENTS.md`; `.agents/rules/*.md` with `trigger: always_on`.
- **every agent**: the memory index `{{paths.memory}}/MEMORY.md`, since the toolkit rule tells every agent to read `{{paths.memory}}`. Counted once when an import already brought it in.

An import outside the project (`@~/…`, an absolute path) is personal: named in the message, not counted. Personal files (`~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`) are not counted either: doctor checks the project.

Budget: **40,960 bytes (40 KiB, about 10k tokens) per agent**. The largest generated AGENTS.md in the samples is 22.5 KB, which leaves ~18 KB for the memory index and rules. Over the budget is a **Warn**, never a Fail: the team decides what to cut.

Message, largest parts first:

```
warn  always-on  claude-code 53.7 KiB over 40.0 KiB: AGENTS.md 34.3, docs/ai/memory/MEMORY.md 19.4, CLAUDE.md 0.0; codex 34.3 KiB of 40.0 KiB; …
ok    always-on  claude-code 15.8 KiB of 40.0 KiB; codex 15.8 KiB of 40.0 KiB
```

`agents-md-size` stays: it answers a different question (an agent truncating AGENTS.md).

## 2. Memory index line limit

**Skill** (`core/skills/continuous-learning/10-continuous-learning.md`, Index section):

- One line, at most **150 characters**: `- [Title](slug.md) - hook`. The hook says when the memory matters, in a few words; the fact, the why and the dates live in the file.
- A line over the limit is shortened when the memory is next touched.

New rule id `learn.index-line-max` in the front matter `rules:`.

**Doctor** check `memory-index`, when `{{paths.memory}}/MEMORY.md` exists:

- Warn when any line is over 150 characters: count, and the first three slugs (`97 lines, 81 over 150 characters: project-tanpa-integrasi-bank-ojk, …`).
- Skip when there is no index.

Doctor never rewrites memory (the toolkit rule: never regenerate or rewrite it). Shortening is the member's job, or the `learn` workflow on the next touch.

## Code

- `internal/doctor/doctor.go`: `alwaysOn(agents)` and `memoryIndex()`, next to `agentsMdSize`. The memory path comes from the profile's `paths.memory` (absolute or relative to the project), as `handoffRosters` reads `paths.plans`.
- Front matter read with the existing YAML helpers; an unreadable front matter counts the file as always on.

## Tests (test first)

1. `internal/doctor`:
   - a fresh sync is OK for each agent, under the budget;
   - a memory index pushing claude-code over 40 KiB warns and names the parts, largest first;
   - `@` imports: followed, each file once, depth 5, outside the project named and not counted;
   - a `.claude/rules` file with `paths:` is not counted, one without is;
   - an Antigravity rule with `trigger: always_on` counts, `model_decision` does not;
   - the memory index is counted once when CLAUDE.md imports it;
   - `memory-index`: a 151-character line warns with its slug, 150 passes, no index skips;
   - `paths.memory` set to another folder is followed.
2. Goldens: `continuous-learning` SKILL.md in every sample and agent, and lock hashes (`test/golden/`); the new doctor lines in the transcripts. Every changed file reviewed.
3. `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`.
4. After: `dashcrew doctor` and `dashcrew usage` on expense_tracker, before and after shortening its index.

## Decisions (maintainer, 2026-11-16)

1. Budget 40 KiB per agent, fixed in code; no profile key.
2. Index line limit 150 characters.

## Result

- Fresh samples: 14.3 to 22.0 KiB per agent, all under the budget.
- expense_tracker as it is: `always-on` warns claude-code 53.7 KiB (AGENTS.md 34.3, memory index 19.4); codex and antigravity 34.3 KiB. `memory-index` does not run: its profile has no `paths.memory`, so dashcrew looks in `.ai/memory`, which does not exist, while the index lives in `docs/ai/memory` and reaches Claude Code only through a hand-added import in CLAUDE.md. Its AGENTS.md tells every agent to read `.ai/memory`.
- With `paths: { memory: docs/ai/memory }` (tried on a copy): every agent 53.7 KiB over, and `memory-index` warns 84 of 95 lines over 150 characters.
- Goldens were rewritten with the `.DS_Store` files moved aside, so lock hashes are those of a clean checkout. The `.DS_Store` problem from P13 is still open.
