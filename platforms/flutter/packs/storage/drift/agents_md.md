---
target: agents_md
order: 340
rules: [storage.drift-one-database, storage.drift-generated-code, storage.drift-versioned-migrations, storage.drift-variables, storage.drift-transactions, storage.access-via-repository]
---
## Local storage: drift

- One `{{pack.storage.database_class}}` (`@DriftDatabase`), opened once and reused; repositories reach it or its DAOs, never widgets.
- Rerun `build_runner` after a table or query change; never edit `*.g.dart`.
- A schema change bumps `schemaVersion` and adds an `onUpgrade` step. Never edit a migration that already shipped.
- Raw SQL (`customSelect`, `customStatement`) takes values through `variables`, never string interpolation.
- Writes that must succeed together share one `transaction`; await every query inside it.
