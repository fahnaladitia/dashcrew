---
target: workflow/verify
order: 25
when: { policy.formatter: false }
---
Never let `dart format` or `dart fix` rewrite files to clear analyzer or diff output; the read-only check in phase 1 is the only formatter call.
