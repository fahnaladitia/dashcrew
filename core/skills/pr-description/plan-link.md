---
target: skill/pr-description
order: 350
when: { policy.plan_gate: true }
rules: [gate.pr-plan-line, gate.pr-closes-plan]
---
- After the last section, two lines naming the branch's plan issue, with the number `dashcrew gate status` prints: `Plan: #<n>` (dashcrew check fails the PR without it, or when that plan is not approved) and `Closes #<n>` (GitHub closes the issue when the PR merges into the default branch).
