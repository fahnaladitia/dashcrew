---
name: data-layer-tdd
description: "Test cases and patterns for Go services and repositories - table-driven tests, hand-written fakes, store tests. Use when: Before writing or changing a service, a repository, a handler's mapping of errors, or a query."
---

# Service and repository TDD

Every feature ships its tests in the same task, not later. The cycle (RED, GREEN, REFACTOR, run budget) is in the `tdd` workflow.

## Service

The service holds the rules, so it is tested the hardest, with a fake repository.

Required cases, one table row each: the happy path, each rule that rejects input, the repository returning an error (wrapped and returned, not swallowed), and the not-found case.

Example: `TestPlaceOrder` with rows `places an order`, `rejects an empty cart`, `wraps the store error`.

## Handler

Test through `httptest`: the request goes in, the status code and body come out. Required cases: valid request, invalid body, the service's not-found and conflict errors mapped to their status codes, and the authorization failure.

## Fakes

A fake is a small struct implementing the consumer's interface, with fields for the values to return and a record of the calls. Never assert on a call order the code does not promise.

## Do not test

- getters, constructors that only assign fields, generated code (`sqlc`, protobuf)
- the standard library or the framework itself

## Repository against a database

- Repository tests run against a real database of the same engine (a container or a local instance) when the repository already has such tests; otherwise the change says how the queries were checked.
- Each test creates the rows it needs and cleans up, or runs inside a transaction that is rolled back.
- A change that needs several writes to succeed together uses one transaction, and the test proves that a failure in the second write leaves the first rolled back.
