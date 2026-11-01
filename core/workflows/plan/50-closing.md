---
target: workflow/plan
order: 50
rules:
  - plan.display-verdict
  - plan.no-code-before-yes
  - plan.reviewer-large-only
---
Display the whole plan in chat as plain text, ending with:

```
Verdict: GO | NO-GO
Reason: <the criterion that failed, or "all criteria pass">
```

## Closing

End every plan with:

```
AWAITING CONFIRMATION: proceed with this plan? (yes / change: ... / other approach: ...)
```

- Write no implementation code before an affirmative answer.
- Tier large: after the yes, run the `reviewer` on the plan. Fix its BLOCKER and HIGH findings in the plan before any code; a change in scope goes back to the user.
