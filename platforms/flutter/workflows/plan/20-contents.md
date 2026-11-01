---
target: workflow/plan
order: 20
rules:
  - plan.acceptance-criteria
  - plan.boundaries
  - plan.verification-map
  - plan.skill-sections
  - plan.package-defaults-from-source
  - plan.affected-files-by-layer
  - plan.skipped-layer-reason
  - plan.existing-code-section
  - plan.real-risks
  - plan.stages-verifiable
  - plan.test-plan
  - plan.leak-analysis
  - plan.dartdoc-targets
  - plan.edge-cases-conflicts
---
## Required sections

### Success criteria and boundaries

- Copy the success criteria confirmed at step 0, with their ids. Add none and drop none without asking.
- Write explicit boundaries in three lists: Always, Ask first, Never.

### Verification map

Map every criterion to the test that proves it (file and test name) or to its manual checklist item. A criterion with neither is a gap the plan must close.

```
AC-1  test/order_mapper_test.dart  maps missing discount to zero
AC-2  manual                       order list after a cold start offline
```

### Skill sections

- `feature-architecture` on the `Skills:` line adds an Architecture section; `design-fidelity` adds a Design section with the design spec.
- Selected packs add their own sections below.

### File map

Map every affected file, grouped by the data-flow layers in AGENTS.md, each marked `[new]` or `[changed]`:

```
request     lib/data/request/order_request.dart          [new]
client      lib/data/client/order_client.dart            [changed]
response    lib/data/response/order_response.dart        [new]
mapper      lib/data/mappers/order_mapper.dart           [new]
model       lib/data/models/order.dart                   [new]
repository  lib/data/repository/order_repository.dart    [changed]
state       lib/ui/orders/state/order_list_state.dart    [new]
ui          lib/ui/orders/order_list_page.dart           [new]
            lib/ui/orders/components/order_item.dart     [new]
di          <dependency registration file>               [changed]
test        test/order_mapper_test.dart                  [new]
            test/order_repository_test.dart              [new]
```

- Name every layer you skip on purpose, with the reason.
- The mapper layer is never skipped.

### Existing code

Search before proposing anything new:

```bash
grep -rn "<concept>" lib/ --include="*.dart" -l | head -20
```

- Report what you found: code to reuse and code that may conflict.
- If nothing is found, show the search commands you ran as proof.
- A plan that builds something that already exists has failed.

### Risks

Focus on real breakage, not hypotheticals:

- shared repositories or models used by other features
- backend response shapes not confirmed by a contract or existing code
- dependency registration order
- local storage schema or migration changes
- package defaults that change behavior (transitions, timeouts, dismissal), checked in the package source and cited with `file:line`

### Stages

- Order the stages so each one can be verified on its own.
- Default order: data layer with its tests, then state, then UI.

### Test plan

- Name each data-layer test and the file it lives in.

### Leak analysis

List every resource the change will create and where it is released:
controllers, focus nodes, timers, stream subscriptions, animation tickers, image loaders.
Write "none" when there are none.

### Dartdoc targets

- List the units this change adds or modifies that the documentation rules require to carry Dartdoc.

### Edge cases and conflicts

- List edge cases (empty, error, partial data, repeated taps, back navigation).
- List conflict risks with existing code and with other open work.
