---
target: agents_md
order: 12
when: { policy.tier_gates: true }
rules: [tier.small-go-continues, tier.standard-or-nogo-waits]
---
Trivial and small run without a plan or another stop. Standard and large stop after the plan: no code until the user answers yes, whatever the verdict.
