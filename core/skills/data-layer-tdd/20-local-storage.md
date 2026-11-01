---
target: skill/data-layer-tdd
order: 20
rules: [storage.find-owner-first, storage.access-from-repository, storage.named-keys, storage.defensive-reads, storage.no-key-rename, storage.no-foreign-store-delete, storage.versioned-cleanup-owner, storage.sensitive-not-plain, storage.tests-temp-dir]
---
## Local storage

When the task touches a local store, a key, or how local data is read:

- Find where stores are opened today and who owns each one; register a new store with its existing owner. Re-check the dependency declarations and store list, because they change.
- Access stores from repositories or the persistence owner, never from UI components.
- Keys are named constants with explicit defaults, and reads are defensive: a stored value may not have the type you expect.
- Never rename a key already stored on user devices without a migration, and never delete or clear a store the task does not own.
- When a store name is versioned, record who cleans up old versions, or put it in the PR notes.
- Tokens, PINs and payment data never go into a plain store; they need encrypted or secure storage.
- Tests open the store in a temporary directory and clear and close it in teardown; a store left open makes other tests flaky. Cover the default for an empty key, a write-read round trip, and a stored value of an unexpected type.
