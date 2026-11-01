---
target: agents_md
order: 340
rules: [storage.access-via-repository, storage.no-key-rename-without-migration]
---
## Local storage: Hive

- Boxes are read and written from repositories or `{{pack.storage.owner_class}}`, never from widgets or entrypoints.
- Never rename a key already stored on user devices without a migration.
