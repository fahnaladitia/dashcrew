---
target: workflow/plan
order: 40
rules:
  - plan.verdict-doubt-is-nogo
  - plan.verdict.open-items
  - plan.verdict.backend-unconfirmed
  - plan.verdict.user-decision
  - plan.verdict.scope-creep
---
## Verdict

Self-gate before closing. The verdict is NO-GO when any criterion below holds. Doubt counts as holding.

- An open item has no owner, or an assumption would change the result if it were wrong.
- The backend response shape is not confirmed by a contract or by existing code.
- A design or approach choice needs a user decision.
- The scope grew beyond the prompt confirmed at step 0.
