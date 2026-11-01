---
target: agents_md
order: 51
when: { policy.formatter: false }
rules: [lint.no-formatter]
---
- Never run a formatter or an auto-fixer as part of a change. Fix analyzer findings by hand, on task lines only.
