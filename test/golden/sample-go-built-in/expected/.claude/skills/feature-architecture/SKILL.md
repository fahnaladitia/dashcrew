---
name: feature-architecture
description: "Decide a feature's structure in a Go service before implementation - packages, layers, interfaces, errors, transactions and goroutines."
when_to_use: "For a new endpoint, service or package, or a change across two or more layers."
---

# Feature architecture

Write these down in the plan before any code.

## Packages and layers

- Which package each new type lives in, following the existing layout, and which layer it belongs to (handler, service, repository).
- The interfaces each consumer needs, with their methods. Prefer extending a consumer's interface to adding a new package.

## Errors

- The domain errors the service returns (`ErrNotFound`, `ErrConflict`, a validation error type) and the status code each maps to in the handler. One mapping place, not one per handler.

## Transactions

- Which operations must succeed together, and where the transaction starts and ends. A transaction never spans an outgoing HTTP call.

## Goroutines and time

- Any background work: who starts it, how it stops, and what happens to it on shutdown. Timeouts for every outgoing call.

## API contract

- Request and response shapes, status codes, and what changes for existing clients. A breaking change is called out with its rollout.
