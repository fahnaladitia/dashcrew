---
target: workflow/verify
order: 25
when: { policy.formatter: false }
---
Never run `dart format` or `dart fix` to clear analyzer or diff output.
