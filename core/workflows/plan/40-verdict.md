---
target: workflow/plan
order: 40
rules:
  - plan.verdict-doubt-is-nogo
  - plan.verdict.open-items
  - plan.verdict.backend-unconfirmed
  - plan.verdict.persisted-or-di-order
  - plan.verdict.user-decision
  - plan.verdict.scope-creep
  - plan.verdict.step0-decision-missing
---
## Verdict

Self-gate before closing. The verdict is NO-GO when any criterion below holds. Doubt counts as holding.

- An open item has no owner, or an assumption would change the result if it were wrong.
- The backend response shape is not confirmed by a contract or by existing code.
- The change touches persisted-data schema or dependency registration order without a decision from the user.
- A design or approach choice needs a user decision.
- The scope grew beyond the step 0 block.
- A step 0 decision the plan depends on is missing.
