---
target: workflow/plan
order: 450
rules: [plan.analytics-section, plan.carry-analytics-decision, plan.verdict.analytics-missing]
---
### Analytics

- The plan always has an Analytics section: does the change add, change, remove or not touch a user action, screen or funnel step? List candidate events from `{{pack.analytics.catalog_doc}}` and recommend asking the user.
- Carry the step 0 analytics decision into the plan. Ask again only when step 0 left it open.
- NO-GO when the step 0 analytics decision is missing.
