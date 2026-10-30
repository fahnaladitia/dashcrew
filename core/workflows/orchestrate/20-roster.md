---
target: workflow/orchestrate
order: 20
rules:
  - roster.display-in-step0
  - roster.decide-before-first-phase
  - roster.nothing-outside
  - roster.evidence-required
  - roster.format
  - roster.saved-to-file
  - roster.add-role-midflow
  - roster.remove-role-needs-yes
  - roster.non-team-flows-same-table
---
## Roster (required, before the first phase)

- Before the first phase, decide every role in the flow chain as RUN or SKIP, using only the role gate triggers below.
- Give every RUN role its active skills, using only the skill gate below.
- Show the roster in the step 0 block, next to the polished prompt, so the user confirms scope, roster and tier once.
- Run no role without a RUN line in the roster.
- Run no skill outside its role's active skills, even when the role definition itself names that skill.
- Non-team flows use the same role gate table for the roles in their chain.

```
ROSTER: <feature>   flow: <flow>   category: feat | fix | hotfix   tier: trivial | small | standard | large
Tier evidence: <files and layers, contracts, ambiguity>
Input: prompt | tracker <id> | PRD <path> | design <link | none>

Role                         Decision  Evidence
product-manager              RUN       no acceptance criteria file for this feature
design-refiner               SKIP      no design link in the prompt
flutter-architect            RUN       new state holder under lib/ui/orders/
flutter-planner              RUN       always
flutter-engineer             RUN       lib/ and test/ change
flutter-code-reviewer        RUN       code changes
qa-engineer                  RUN       new QA identifiers on order_list_page
flutter-security-reviewer    SKIP      no auth, storage, network, deep link or analytics in scope
tech-lead                    RUN       2 review roles run + acceptance criteria from product-manager

Active skills
product-manager:             idea-refine, user-story, deliver-acceptance-criteria
flutter-engineer:            flutter-expert, flutter-development, data-layer-tdd
qa-engineer:                 verification-loop, flutter-add-widget-test
```

- Evidence names a file, an input or a path that exists or does not exist.
- "Seems unnecessary" and "usually used" are not evidence; a decision without evidence is invalid.
- Save the roster to `{{paths.plans}}/<feature>/handoff/00-roster.md`.
- Every role prompt carries that path and the role's own active skills line.

## Changing the roster mid-flow

- A role may be added without asking when a running role requests it under "For next phase" with evidence. Update the roster and tell the user in one line.
- Removing a role already marked RUN needs an explicit yes from the user.
