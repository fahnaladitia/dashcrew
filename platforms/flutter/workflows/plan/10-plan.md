---
target: workflow/plan
order: 10
rules:
  - skill.plan-phase
  - plan.official-docs
  - plan.restate
  - plan.one-file
  - plan.code-md-large-only
  - plan.code-md-no-test-bodies
  - plan.reuse-search-explorer
---
# plan

The main session writes the plan for tier standard and large{{^policy.tier_gates}}, and for small{{/policy.tier_gates}}. No implementation code is written until the plan is accepted (see Closing).

## Skills

- Apply the skills on the step 0 `Skills:` line; `flutter-expert`, `flutter-development`, `spec-driven-development` and `source-driven-development` when installed.
- Ground every framework and package decision in the official Flutter, Dart or package documentation, and read the package source where a default matters.
- Never guess an API, and never plan around a deprecated one.

## Restate

- Restate the requirement in your own words.
- Add no feature the step 0 block did not confirm.
- List every ambiguity as an explicit assumption.

## Search first

- Search for the concept, the feature folder and the entities involved before proposing anything new.
- When the answer needs a sweep across many files, ask the `explorer` for a `file:line` map instead of reading them all here.

## File

Save the plan to `{{paths.plans}}/<task-slug>/plan.md`, one file.

- Tier large adds `code.md` next to it: signatures and the hunks that carry a decision, nothing else.
- `code.md` never holds full test bodies; they are written test first during implementation.
