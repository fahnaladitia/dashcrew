---
target: agents_md
order: 13
when: { policy.plan_gate: true }
rules: [gate.plan-on-issue, gate.no-self-approval]
---
Plan gate: when the plan waits for the user, post it with `dashcrew gate submit plan --file <plan.md>` and stop. The user approves on the GitHub issue; resume only when `dashcrew gate status` says approved. Until then the guard refuses writes under `lib/` and `test/`. Never post `/approve` yourself. The PR description ends with `Plan: #<n>` and `Closes #<n>` for that issue.
