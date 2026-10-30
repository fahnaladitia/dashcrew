---
target: workflow/plan
order: 50
rules:
  - plan.display-verdict
  - plan.no-code-before-yes
---
Display the whole plan in chat as plain text, ending with:

```
Verdict: GO | NO-GO
Reason: <the criterion that failed, or "all criteria pass">
```

## Closing

{{#policy.tier_gates}}
When step 0 set the tier to `trivial` or `small` and the verdict is GO, end with:

```
CONTINUE: tier <tier>, verdict GO. Implementation starts without confirmation.
```

In every other case (tier `standard` or `large`, or NO-GO on any tier), end with:
{{/policy.tier_gates}}
{{^policy.tier_gates}}
End every plan with:
{{/policy.tier_gates}}

```
AWAITING CONFIRMATION: proceed with this plan? (yes / change: ... / other approach: ...)
```

Write no implementation code before an affirmative answer{{#policy.tier_gates}}, except in the CONTINUE case above{{/policy.tier_gates}}.
