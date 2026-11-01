---
name: qa-identifiers
description: "Propose, apply and audit Semantics QA identifiers, and find widgets by them in tests. Use when: When a task adds, changes or removes QA-relevant widgets, or writes widget or integration tests."
---

# QA identifiers

The identifier rules are in AGENTS.md. This skill covers proposing, applying and auditing them.

## Propose and apply

- Propose an identifier for each QA-relevant widget as `<screen>_<element>_<role>`, in the plan's architecture or design section. Implementation takes names from that list.
- Before assigning one, search the code and tests to confirm the name is not already used.
- Tests find actions with `find.byKey(const ValueKey('<identifier>'))`, not `find.text`. Integration journeys target the same identifiers.
- If the project keeps a central identifier registry or an automation coverage document and the screen appears in either, update both.

## Review checklist

On the diff, check every identifier:

- `key` and `identifier` carry the same string.
- The name follows the convention and holds no dynamic value (index, hash code, timestamp).
- Every QA-relevant widget a sliced or redesigned screen shows has one.
- No identifier present before the diff is lost or renamed: compare removed identifier lines. A removed one is HIGH until a search of the automation suites shows it unused.
