---
target: workflow/orchestrate
order: 30
rules:
  - roster.trigger.product-manager
  - roster.trigger.design-refiner
  - roster.trigger.flutter-architect
  - roster.trigger.flutter-planner
  - roster.trigger.flutter-engineer
  - roster.trigger.flutter-code-reviewer
  - roster.trigger.qa-engineer
  - roster.trigger.flutter-security-reviewer
  - roster.trigger.tech-lead
---
## Role gate

These triggers are the only basis for RUN or SKIP.

| Role | RUN when | SKIP when |
|---|---|---|
| `product-manager` | no checkable acceptance criteria exist: no acceptance criteria file for the feature, and neither the prompt nor the tracker task states explicit criteria | valid acceptance criteria already exist; or the category is fix or hotfix and the prompt already holds the reproduction and the expected result |
| `design-refiner` | the user gave a design link in the prompt | there is no design link; the role never searches for designs itself, the UI follows comparable merged components, and the plan records "no design" |
| `flutter-architect` | a new screen, a new state holder, a new feature folder, or a change across two or more layers | the change stays inside an existing widget or state holder with no new state boundary; copy; a one-field mapper; a fix backed by an audit under `{{paths.audits}}/` with a `file:line` trace and a proposed fix, adding no new state or screen (the planner reads the audit directly) |
| `flutter-planner` | always in the `team` flow | never |
| `flutter-engineer` | `lib/` or `test/` changes | never when code changes |
| `flutter-code-reviewer` | code changed | never when code changed |
| `qa-engineer` | QA identifiers are added, changed or removed; or product-manager acceptance criteria must map to tests; or the user asks for widget or integration tests | pure data-layer work whose tests flutter-engineer already wrote, with no new criteria and no identifier change |
| `flutter-security-reviewer` | scope touches any of: auth, token or session; local storage; network client or config; deep links; WebView; analytics properties; permissions; platform manifests or entitlements; logging of user data | none of those: UI slicing, copy, layout, mappers without sensitive data |
| `tech-lead` | two or more review roles run, or product-manager or design-refiner handoffs need a spec check | only flutter-code-reviewer runs and there is no PRD and no design spec; the code-reviewer verdict is the gate |

Selected packs add rows for their own roles.
