---
target: agents_md
order: 51
when: { policy.formatter: true }
---
- Run the project formatter{{#platform.commands.format}} (`{{.}}`){{/platform.commands.format}} only on the files you changed, never across the repo.
