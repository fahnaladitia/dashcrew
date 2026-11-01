---
target: workflow/code-review
order: 20
rules:
  - review.priorities
  - review.removed-identifier-high
---
## Priorities

Review in this order: correctness, lifecycle and async; architecture boundaries; test coverage; QA identifiers; performance.

**BLOCKER**

- `BuildContext` used after an `await` without a `mounted` guard
- a controller, focus node, stream subscription or state holder that is never disposed
- a response DTO reaching the UI without going through a mapper
- a hardcoded credential
- `!` on a value not proven non-null
- `dynamic` in new code

**HIGH**

- a new mapper, repository, model or state holder without a test
- business logic in a widget instead of the state holder
- a QA-relevant widget without its identifier, or an identifier built from a dynamic value (list index, hashCode, timestamp)
- an identifier removed or renamed without proof that no automation suite uses it

**MEDIUM**

- a missing `const` constructor
- a large subtree rebuilding where a narrower rebuild scope would do
- `print(` instead of the project logger
- a widget `build` method over 100 lines
- a magic number without a named constant

Selected packs add stack-specific items right after this list.
