---
target: workflow/verify
order: 21
when: { policy.formatter: true }
rules: [verify.format-fix-formatter]
---
Run `dart format` on the listed files only, then run the check again.
