---
target: workflow/orchestrate
order: 340
rules: [roster.trigger.local-storage-engineer, roster.skill.local-storage-engineer]
---
### Roster: local-storage-engineer

- Runs only when the scope touches Hive: opening a box, a new key, a TypeAdapter, a default value, or a migration. Skipped when no Hive code is in scope.
- Active skill: `managing-hive-storage`, always, as API reference only.
