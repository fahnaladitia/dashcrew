---
name: data-layer-tdd
description: "Test cases and patterns for data-layer TDD - mappers, repositories, models and state holders."
when_to_use: "Before writing or changing a mapper, repository, model, state holder or local store."
---

# Data-layer TDD

Every feature ships data-layer tests in the same task, not later.
The cycle (RED, GREEN, REFACTOR, run budget) is in the `tdd` workflow.
The test-double style (hand-written fakes or a mocking library) is set by the selected test pack; follow AGENTS.md.

## Fixtures

- Use the real JSON fixtures in the project's fixture folder.
- Never invent a response shape. A test that passes on a wrong assumption is worse than no test.

## Mapper

The mapper is the one place where the backend shape becomes the domain model, so it is tested the hardest.

Required cases:

- full response, every field present
- optional fields that are `null` map to their defaults
- fields missing from the JSON entirely
- empty lists
- numbers sent as strings

Example names: `maps full response` (read `<fixture folder>/order_detail.json`, map it, assert the id and the total), `maps missing discount to zero` (map `{"id": "1"}`, assert the discount is 0).

## Repository

Use a test double for the client. Required cases: success, client error, empty response.

Example names: `returns mapped order on success`, `propagates client error`.

## Model

A model with value equality gets two tests that prove its equality covers every field: `equal when fields are equal`, and `not equal when id differs`.

This is not a trivial test. An equality that misses a field is why a UI does not refresh when the data changed.

## State holder

Test the sequence of emitted states. Always cancel the subscription and close the state holder at the end of the test; leaks make other tests flaky.

## Do not test

- getters that only pass a field through
- the framework itself
- generated code
- anything that passes whatever the implementation is

## Local storage

When the task touches a local store, a key, or how local data is read:

- Find where stores are opened today and who owns each one; register a new store with its existing owner. Re-check the dependency declarations and store list, because they change.
- Access stores from repositories or the persistence owner, never from UI components.
- Keys are named constants with explicit defaults, and reads are defensive: a stored value may not have the type you expect.
- Never rename a key already stored on user devices without a migration, and never delete or clear a store the task does not own.
- When a store name is versioned, record who cleans up old versions, or put it in the PR notes.
- Tokens, PINs and payment data never go into a plain store; they need encrypted or secure storage.
- Tests open the store in a temporary directory and clear and close it in teardown; a store left open makes other tests flaky. Cover the default for an empty key, a write-read round trip, and a stored value of an unexpected type.
