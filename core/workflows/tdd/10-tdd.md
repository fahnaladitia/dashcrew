---
target: workflow/tdd
order: 10
rules:
  - skill.tdd-phase
  - tdd.data-layer-test-first
  - tdd.ui-after-data-layer
  - tdd.ui-tests-on-request
---
# tdd

Apply `test-driven-development` and `data-layer-tdd`. The test cases each unit type needs are in `data-layer-tdd`.

## Scope

- Write data-layer logic test first: mappers, repositories, models, and state holders.
- Implement in this order: mapper, model, repository, state holder, then UI.
- Start the UI only after the data-layer tests pass.
- Skip UI tests unless the user or the plan asks for them. The gap stays visible in the verification map.
- Read every file in full before changing it.
