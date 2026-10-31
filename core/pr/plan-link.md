---
target: pr_template
order: 950
when: { policy.plan_gate: true }
rules: [gate.pr-plan-line, gate.pr-closes-plan]
---
<!-- The plan issue number comes from dashcrew gate status. "Plan:" is checked by dashcrew check; "Closes" closes the issue when this PR merges into the default branch. -->
Plan: #<issue>
Closes #<issue>
