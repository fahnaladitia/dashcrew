---
name: backend-security
description: "Security checklist for a Go backend - queries, authorization on handlers, secrets in config, migrations. Use when: When the change adds or edits a handler, a query, a migration, configuration, or anything that reads credentials."
---

# Backend security

The always-on rules (parameters, secrets, logs, authorization, timeouts, body limits) are in AGENTS.md. This checklist adds what a reviewer walks through on the diff.

## Queries

- Every value from a request reaches the store as a parameter. Identifiers that cannot be parameters (a sort column, a table) come from an allowlist in code, never from input.
- A list endpoint has a maximum page size.
- A query that changes rows has a `WHERE` that the test proves; an update or delete without one is a blocker.

## Authorization

- For each new or changed handler, name who may call it and where that is checked. A resource looked up by id is checked against the caller (owner, tenant, role), not only for existence.
- Errors do not reveal whether a resource the caller may not see exists: use the same response as not found.

## Secrets and configuration

- No secret in source, test fixtures, committed `.env` files or default config values.
- A new setting has a safe default for production, or startup fails without it.

## Migrations

- Each migration has a working down step, or the PR says why it cannot (a destructive change) and how to restore.
- A migration that rewrites a large table, adds a non-null column without a default, or drops a column in use is called out with a rollout order.
- Agents run migrations against local databases only; the guard blocks remote ones.

## Verdict

List each finding with `file:line`, severity (BLOCKER, HIGH, MEDIUM, LOW) and the fix. No finding: say which sections were checked.
