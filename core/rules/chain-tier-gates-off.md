---
target: agents_md
order: 12
when: { policy.tier_gates: false }
rules: [tier.gates-off-every-plan-waits]
---
Every tier except trivial writes a plan, small included, and every plan waits: touch no code until the user answers yes.
