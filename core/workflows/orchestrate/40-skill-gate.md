---
target: workflow/orchestrate
order: 40
rules:
  - roster.skill-gate
  - roster.skill.product-manager
  - roster.skill.design-refiner
  - roster.skill.flutter-architect
  - roster.skill.flutter-planner
  - roster.skill.flutter-engineer
  - roster.skill.qa-engineer
  - roster.skill.flutter-security-reviewer
  - roster.skill.none-for-reviewers
  - roster.skill-used-skipped
---
## Skill gate

- For each RUN role, write its active skills in the roster and in the role prompt.
- An "always" skill runs every time the role runs.
- A "trigger" skill runs only when its condition holds, and the condition is written in the roster as evidence.

| Role | Always | Trigger |
|---|---|---|
| `product-manager` | `idea-refine` | `prd` when the feature has no PRD yet; `user-story` and `deliver-acceptance-criteria` when acceptance criteria are missing |
| `design-refiner` | the design-to-code skill of the selected design pack, loaded before the first design-context fetch | none |
| `flutter-architect` | none | the state-management patterns skill of the selected state pack, only when a state holder is new or its state changes; `codebase-design` only for a new module or seam (new feature folder, new repository) |
| `flutter-planner` | `spec-driven-development` | `source-driven-development` when the plan uses a Flutter or package API with no example in the repo yet |
| `flutter-engineer` | `flutter-expert`, `flutter-development` | `data-layer-tdd` when touching a mapper, repository, model or state holder; `flutter-fix-layout-issues` only on an overflow or unbounded-constraint error; `flutter-build-responsive-layout` only when acceptance criteria name tablet, orientation or breakpoints |
| `qa-engineer` | `verification-loop` | `qa-expert` when product-manager ran; `flutter-add-widget-test` when writing widget tests; `flutter-add-integration-test` only when the user asks for a journey test |
| `flutter-security-reviewer` | the OWASP Mobile Top 10 reference and the project security rules | the OWASP scanner only when scope touches storage, network or credentials; `security-and-hardening` only for WebView, deep link or URL input, or form validation |

- `flutter-code-reviewer` and `tech-lead` have no skills.
- A role not listed here has no skills, unless a selected pack adds a row.
- Every role closes its HANDOFF with "Skills used" and "Skills skipped" (or "none").
- A used skill outside the role's active skills is a process violation; tech-lead records it.
