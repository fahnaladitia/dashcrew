---
target: agents_md
order: 51
when: { policy.formatter: false }
rules: [lint.no-formatter]
---
- Never let a formatter or auto-fixer (`dart format`, `dart fix`) rewrite files. Fix analyzer findings by hand, on task lines only.
