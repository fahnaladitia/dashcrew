---
target: role/local-storage-engineer
order: 300
rules: [storage.skill-api-reference-only, storage.recheck-facts, storage.box-registered-with-owner, storage.no-per-feature-typeadapter, storage.named-keys-defaults, storage.versioned-box-cleanup, storage.auth-box-escalate, storage.test-pattern, storage.no-destructive-foreign-boxes, storage.type-change-raise, storage.regenerate-on-hivetype, role.local-storage-engineer.handoff]
---
## Hive

### Before you start

- Use the `managing-hive-storage` skill as an API reference only. Its install, `initFlutter` and `@HiveType` setup phases do not apply; the facts of this repository win.
- Re-check the repository facts before relying on them: the `{{pack.storage.package}}` dependency in `pubspec.yaml`, where `{{pack.storage.owner_class}}` opens boxes, and every `Hive.box(` call site.

### Rules

- A new box is opened by `{{pack.storage.owner_class}}` at startup. `Hive.box()` on a box that was never opened crashes at runtime.
- Boxes are untyped. Store primitives, maps or JSON strings, and map them in the repository. Never start a `@HiveType` for one feature.
- Keys are named constants. Every read passes an explicit `defaultValue` and casts defensively (`box.get(key) as int? ?? 0`).
- A box per app version names who deletes the previous version's box; if nobody does, list it under Open items in the handoff.
- Any change to the `{{pack.storage.auth_box}}` box is escalated to flutter-security-reviewer. Never widen what it stores silently.
- Never `deleteFromDisk()` or `clear()` a box the task does not own.
- Changing a `typeId` or the field order of a Hive type already in use breaks data on installed devices. Raise it to the user before writing code.
- Adding a `@HiveType` or changing adapter fields or a `typeId` requires running build_runner.

### Tests

- Init Hive in a temporary directory, open the box inside the test, and clear and close it in `tearDown`.
- Cover three cases: a missing key returns its default, a value round-trips, and an unexpected stored type falls back safely.

### Handoff

- Decided: the box owner and why, and the stored data shape (primitive, map or JSON string).
- Open items: pending migrations and old-version cleanup, each with an owner.
- Next phase: for security, what is stored where; for QA, the cold-start behavior to check.
