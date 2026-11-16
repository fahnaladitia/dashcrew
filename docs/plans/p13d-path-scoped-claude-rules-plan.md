# P13d: path-scoped Claude Code rules for the stack packs

Tier: large (output layout change: new `.claude/rules/` files, AGENTS.md or CLAUDE.md content per agent, pack front matter). Goldens rewritten and reviewed before merge.

Last of the four token-saving tasks (P13 usage, P13b budget gate, P13c lean execution, P13d this).

## Goal

Claude Code loads a stack pack's rules (state, di, router, network, analytics, storage, test, qa, and the platform's `Flutter and Dart` rules) only when it reads or edits a matching file, through `.claude/rules/<name>.md` with `paths:` front matter. Codex and Antigravity keep the full AGENTS.md.

## What it saves (expense_tracker, measured)

| Always-on part for Claude Code | Bytes | Movable by P13d |
|---|---|---|
| Generated AGENTS.md | 20,412 | ~5,300: State 1,136, Flutter and Dart 942, Analytics 941, DI 800, Navigation 585, Libraries without a pack 574, Tests 176, Network 167 |
| Hand-written project section of AGENTS.md | 14,697 | no |
| Memory index (imported by hand in CLAUDE.md) | 19,891 | no (P13b's 150-character lines shrink it) |
| Total | ~55,000 | ~5,300, about 10% (~1.3k tokens) |

Two limits on that 10%:

- **Most pack rules apply to most Dart files.** "Never `context.read()` inside `build`" applies to every widget, not just `bloc/`. Analytics rules apply wherever an event fires. Safe globs end up close to `lib/**/*.dart`, so the rules still load in nearly every coding session. They stay out only in sessions that touch no Dart file (planning, docs, review of non-Dart files). Narrower globs save more but let a rule miss a file it applies to, and some of these rules are review blockers.
- **Some pack rules are step 0 rules, not file rules.** Analytics' "on every feature change, flag the event and ask" has to stay always-on. Each pack fragment would split into an always-on part and a scoped part.

## The design problem: Claude reads AGENTS.md whole

`CLAUDE.md` is `@AGENTS.md`, so whatever AGENTS.md holds, Claude Code loads. Codex has no imports and must get the rules inline. Leaving the packs out for Claude alone means Claude gets its own file:

- **A. Generated CLAUDE.md.** When dashcrew owns CLAUDE.md, it is written as the AGENTS.md body without the scoped fragments, plus a copy of the project section taken from AGENTS.md at sync. Cost: ~20 KB of generated text in two files. An edit to the project section reaches Claude only after `dashcrew sync`; until then doctor `stale` flags CLAUDE.md. **expense_tracker gains nothing as it is:** its CLAUDE.md was edited by hand (the memory import), so dashcrew does not own it and leaves it alone.
- **B. Claude-only projects.** Only when `agents: [claude-code]`: AGENTS.md is read only by Claude, so the scoped fragments leave AGENTS.md and go to `.claude/rules/`. No duplication, no staleness. A project with Codex or Antigravity (expense_tracker) is unchanged.
- **C. Not now.** The movable part is about 10% and mostly read from cache after the first turn (billed at a tenth). The larger always-on parts are not pack rules: the hand-written project section (14.7 KB; in expense_tracker it repeats generated sections: Git, Branching and Testing under the same heading, and red flags, the workflow chain, code, documentation, debugging and analytics under a similar one) and the memory index (19.9 KB). A cheaper follow-up for those: doctor `always-on` names the project-section headings that repeat a generated heading exactly (catches Git, Branching and Testing; similar names are left to the team).

## Recommendation

C now, with the follow-up check above as a small task. Revisit A after `dashcrew usage` shows how often sessions touch no Dart file at all. A only pays off when that is frequent, and only in repos where dashcrew owns CLAUDE.md.

If A or B goes ahead, the shared parts:

- Pack front matter gains `paths:` (globs rendered with the pack params, such as `{{pack.state.feature_dir}}`). A pack with `paths:` splits its `agents_md.md` into an always-on part and a scoped part.
- The Claude Code adapter writes `.claude/rules/<family>-<option>.md` (`dashcrew` marker, `paths:` front matter) and owns them through the lock like other generated files.
- `doctor` `always-on` (P13b) already skips `.claude/rules` files that have `paths:`.
- Tests: adapter output per option, lock ownership, a removed pack deletes its rule file, AGENTS.md unchanged for Codex and Antigravity; goldens for every sample with Claude Code; `TestLargestOptionOfEveryFamilyStaysUnderTheLimit` still holds.

## Decision (maintainer, 2026-11-16)

C. Path-scoped rules are deferred; revisit with `dashcrew usage` data. The follow-up runs as a small task: doctor `always-on` names the headings of the AGENTS.md project section that repeat a generated heading exactly (case-insensitive), without changing the check's status.

## Result (the small follow-up)

- `internal/doctor/budget.go` `repeatedHeadings`, appended to the `always-on` detail; the status is unchanged. Test in `budget_test.go`.
- expense_tracker: `the AGENTS.md project section repeats generated headings: Git, Branching, Testing`.
- No golden changed: no sample has a project section that repeats a heading.
