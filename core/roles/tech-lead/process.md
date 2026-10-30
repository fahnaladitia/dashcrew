---
target: role/tech-lead
order: 200
rules: [role.delegation-role-based, role.no-generic-agent, role.single-file-main-session, skill.role-skill-catalog, skill.excluded-skills]
---
## Delegation checks

Flag these as process violations on the Standards axis:

- Flutter or Dart work in the team flow that was not delegated to the named role for it.
- Flutter or Dart work handed to a generic or ad-hoc agent instead of a named role.
- A subagent spawned for a single-file fix or a copy change; those stay in the main session.

## Role skill map

Use this map to judge `Skills used:` lines. The roster's active skills narrow it per task; the selected packs add stack-specific skills.

| Role | Skill kinds |
|---|---|
| product-manager | PRD writing, user stories, acceptance criteria, idea refinement |
| design-refiner | the design pack's design-to-code skill |
| flutter-architect | module and seam design; the state pack's patterns skill |
| flutter-planner | spec-driven and source-driven development |
| flutter-engineer | Flutter development, data-layer TDD, layout-error fixing, responsive layout |
| local-storage-engineer | the storage pack's API reference skill |
| qa-engineer | QA strategy, verification loop, widget tests, integration tests |
| flutter-security-reviewer | OWASP Mobile Top 10 checker; web hardening only for web views, URL input or form validation |
| flutter-build-error-resolver | package-conflict resolution |
| any role debugging a running app | runtime-error diagnosis; the fix still goes through the chain |

## Excluded skills

A skill used in a handoff is a violation when it conflicts with the project's choices:

- architecture skills that impose a state-management pattern the project does not use
- test skills that require a mocking library the project does not use
- long-form documentation writers that break the Dartdoc line cap
- mass lint or format auto-fix skills
- setup skills for an HTTP client, router or analytics product the project does not use
- retired duplicates of a skill already in the map above
