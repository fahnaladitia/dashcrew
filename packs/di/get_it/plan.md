---
target: workflow/plan
order: 410
rules: [plan.verdict.di-order-change]
---
### Dependency injection

- NO-GO when the change reorders existing registrations in `{{pack.di.container_file}}`.
