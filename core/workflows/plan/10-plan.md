---
target: workflow/plan
order: 10
rules:
  - skill.plan-phase
  - plan.official-docs
  - plan.restate
  - plan.save-two-files
  - plan.code-md-no-test-bodies
  - plan.planner-writes-own-files
---
# /plan

Runs the `flutter-planner` role. No implementation code is written until the plan is accepted (see Closing).

## Skills

- Apply `flutter-expert`, `flutter-development`, `spec-driven-development` and `source-driven-development`.
- Ground every framework and package decision in the official Flutter, Dart or package documentation.
- Never guess an API, and never plan around a deprecated one.

## Restate

- Restate the requirement in your own words.
- Add no feature the prompt did not ask for.
- List every ambiguity as an explicit assumption.

## Files

Save the plan to `{{paths.plans}}/<plan-folder>/` as two files:

- `plan.md`: requirements, strategy, architecture, affected files, edge cases, leak analysis.
- `code.md`: the concrete code changes to `lib/` per file, plus a test list (file, test name, setup, main assertion).

- `code.md` never holds full test bodies; the implementing role writes them.
- `flutter-planner` writes `plan.md` and `code.md` itself. The main session never retypes their content.
