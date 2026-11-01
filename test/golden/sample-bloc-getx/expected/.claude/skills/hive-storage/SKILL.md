---
name: hive-storage
description: "Hive specifics for this project - who opens boxes, keys and defaults, schema changes and tests."
when_to_use: "When the scope touches Hive - opening a box, a new key, a TypeAdapter, a default value or a migration."
---

# Hive

## Before you start

- Use the `managing-hive-storage` skill, when installed, as an API reference only. Its install, `initFlutter` and `@HiveType` setup phases do not apply; the facts of this repository win.
- Re-check the repository facts before relying on them: the `hive_ce` dependency in `pubspec.yaml`, where `LocalDatabase` opens boxes, and every `Hive.box(` call site.

## Rules

- A new box is opened by `LocalDatabase` at startup. `Hive.box()` on a box that was never opened crashes at runtime.
- Boxes are untyped. Store primitives, maps or JSON strings, and map them in the repository. Never start a `@HiveType` for one feature.
- Keys are named constants. Every read passes an explicit `defaultValue` and casts defensively (`box.get(key) as int? ?? 0`).
- A box per app version names who deletes the previous version's box; if nobody does, list it in the PR notes.
- Any change to the `auth` box puts `mobile-security` on the step 0 `Skills:` line. Never widen what it stores silently.
- Never `deleteFromDisk()` or `clear()` a box the task does not own.
- Changing a `typeId` or the field order of a Hive type already in use breaks data on installed devices. Raise it to the user before writing code.
- Adding a `@HiveType` or changing adapter fields or a `typeId` requires running build_runner.
- The plan records the box owner and why, the stored data shape (primitive, map or JSON string), pending migrations or old-version cleanup with an owner, and the cold-start behavior with empty boxes to check.

## Tests

- Init Hive in a temporary directory, open the box inside the test, and clear and close it in `tearDown`.
- Cover three cases: a missing key returns its default, a value round-trips, and an unexpected stored type falls back safely.
