---
target: workflow/plan
order: 55
when: { policy.plan_gate: true }
rules: [gate.plan-on-issue]
---
## Plan gate

When the plan waits for the user's confirmation, post it to the branch's GitHub issue and stop:

```bash
dashcrew gate submit plan --file {{paths.plans}}/<feature>/plan.md
```

Report the issue link it prints. When the user says they approved, run `dashcrew gate status`; continue only when it prints `plan: approved`.
