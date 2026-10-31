---
name: pr-description
description: "Write or update the single PR description file of the current branch from its real diff."
when_to_use: "At step 7 of every finished round of work on a branch, before asking about opening a PR."
---

# PR description

Each branch has exactly one PR description file. Write it at step 7 from the diff you still have in context, not later when the PR is opened, and update it on every later round of work on the same branch.

## Path

- `docs/pr/<branch>.md`, where `<branch>` is the verbatim output of `git branch --show-current`. A `/` stays a folder: `feat/order-history` writes `docs/pr/feat/order-history.md`.
- One file per branch for its whole life: update the existing file, never open a second one, never rename it.

## Content

- The source of truth is the real diff, not the plan: `git diff <base>...HEAD --stat` plus the file list.
- Anything planned but dropped or deferred is written down as dropped.
- Write it in language `id` (ISO 639-1 code), with technical terms kept in English (merge, endpoint, payload, state, widget).
- Updating merges into the existing sections. Never append a "Round 2" block, and never rewrite history that is still true.
- Every section stays even when empty: write "None", so a reviewer can see the question was asked.
- The file ends at the last section. No generated-by footer.

## Sections, in this order

Keep every heading exactly as in `.github/pull_request_template.md`, in English, even when the body is in another language: `dashcrew check` fails a PR without one.

- `# <type>: <short title>` - same style as the branch name.
- `## Summary` - one paragraph: the problem solved and the end result.
- `## Changes` - grouped by area (data layer, state, UI, tests), not by file.
- `## Affected files` - each path with what changed in it.
- `## Testing` - the scoped commands that ran (`flutter analyze <paths>`, `flutter test <files>`) and their results. Without tests: why, and how the change was verified.

- `## Notes / not done` - always last: deliberate omissions, follow-ups, remaining risks, design gaps, open backend questions.
