---
target: agents_md
order: 12
when: { policy.tier_gates: true }
rules: [tier.small-go-continues, tier.standard-or-nogo-waits]
---
After the plan: trivial or small with Verdict GO continues to step 3 without waiting. Standard or large, or NO-GO on any tier, stops and waits; touch no code until the user answers yes.
