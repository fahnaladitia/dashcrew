---
target: workflow/plan
order: 440
rules: [plan.verdict.storage-type-change]
---
### Local storage

- NO-GO when the plan changes a `@HiveType` (new type, `typeId`, or field order).
