---
target: workflow/verify
order: 21
when: { policy.formatter: false }
rules: [verify.format-fix-by-hand]
---
Format the listed lines by hand when they belong to the task, then run the check again. A listed file whose differences are all outside the task is reported, and the user decides.
