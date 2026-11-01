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

- UI or state touched after an `await` once its owner may be gone, without a guard
- a subscription, listener, timer, connection or state holder that is never closed or cancelled
- a response DTO reaching the UI without going through a mapper
- a hardcoded credential
- a forced unwrap or cast of a value not proven non-null
- an untyped value in new code where the language allows a type

**HIGH**

- a new mapper, repository, model or state holder without a test
- business logic in a UI component instead of the state holder
- a QA-relevant element without its identifier, or an identifier built from a dynamic value (list index, hash code, timestamp)
- an identifier removed or renamed without proof that no automation suite uses it

**MEDIUM**

- a large part of the UI re-rendering where a narrower scope would do
- debug output instead of the project logger
- a function over 100 lines
- a magic number without a named constant

Selected packs add stack-specific items right after this list.
