---
target: agents_md
order: 340
rules: [storage.prefs-new-api, storage.access-via-repository, storage.prefs-non-critical-only, storage.no-key-rename-without-migration, storage.prefs-supported-types]
---
## Local storage: shared_preferences

- New code uses `SharedPreferencesAsync`, or `SharedPreferencesWithCache` with an allowlist; never the legacy `SharedPreferences` API.
- Reads and writes go through repositories or `{{pack.storage.owner_class}}`, never widgets or entrypoints.
- Store only settings and flags the app can afford to lose: writes are not guaranteed to reach disk. Never critical data, tokens or personal data.
- Never rename a key already stored on user devices without a migration.
- Supported values are `int`, `double`, `bool`, `String` and `List<String>`; anything else is encoded explicitly.
