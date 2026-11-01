---
target: skill/pr-description
order: 100
rules: [pr.description-path, pr.one-file-per-branch, pr.write-at-step7, pr.from-diff-not-plan, pr.record-dropped, pr.language, pr.ends-at-last-section, pr.update-merges-sections, pr.sections-never-deleted, pr.section.title, pr.section.summary, pr.section.changes, pr.section.files, pr.section.testing, pr.testing-manual-checklist]
---
# PR description

Each branch has exactly one PR description file. Write it at the end of the chain from the diff you still have in context, not later when the PR is opened, and update it on every later round of work on the same branch.

## Path

- `{{paths.pr}}/<branch>.md`, where `<branch>` is the verbatim output of `git branch --show-current`. A `/` stays a folder: `feat/order-history` writes `{{paths.pr}}/feat/order-history.md`.
- One file per branch for its whole life: update the existing file, never open a second one, never rename it.

## Content

- The source of truth is the real diff, not the plan: `git diff <base>...HEAD --stat` plus the file list.
- Anything planned but dropped or deferred is written down as dropped.
- Write it in language `{{language.pr}}` (ISO 639-1 code), with technical terms kept in English (merge, endpoint, payload, state, component).
- Updating merges into the existing sections. Never append a "Round 2" block, and never rewrite history that is still true.
- Every section stays even when empty: write "None", so a reviewer can see the question was asked.
- The file ends at the last section. No generated-by footer.

## Sections, in this order

Keep every heading exactly as in `.github/pull_request_template.md`, in English, even when the body is in another language: `dashcrew check` fails a PR without one.

- `# <type>: <short title>` - same style as the branch name.
- `## Summary` - one paragraph: the problem solved and the end result.
- `## Changes` - grouped by area (data layer, state, UI, tests), not by file.
- `## Affected files` - each path with what changed in it.
- `## Testing` - the scoped commands that ran (`{{platform.commands.analyze}} <paths>`, `{{platform.commands.test_scoped}} <files>`) and their results. Without tests: why, and how the change was verified. The manual checklist from step 0 goes here too, each item with the user's result.
