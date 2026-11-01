---
target: agents_md
order: 340
rules: [storage.sqflite-one-database, storage.sqflite-versioned-migrations, storage.sqflite-placeholders, storage.sqflite-txn-only, storage.sqflite-types, storage.access-via-repository]
---
## Local storage: sqflite

- One `Database`, opened by `{{pack.storage.owner_class}}` with a `version` and reused; repositories reach it through that class, never widgets.
- A schema change bumps `version` and adds an `onUpgrade` step. Never edit a migration that already shipped.
- Values go through `?` placeholders and `whereArgs`, never string interpolation.
- Inside `transaction`, use only the `txn` argument; using the database object there deadlocks.
- No `bool` or `DateTime` columns: store 0/1 and milliseconds or ISO 8601 text. Query results are read-only; copy a row before changing it.
