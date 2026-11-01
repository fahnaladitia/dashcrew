---
target: skill/data-layer-tdd
order: 10
rules:
  - tdd.mapper-cases
  - tdd.repository-cases
  - tdd.equatable-model-tests
  - tdd.real-fixtures
  - tdd.no-trivial-tests
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
