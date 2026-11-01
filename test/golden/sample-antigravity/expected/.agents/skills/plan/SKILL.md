---
name: plan
description: "Plan a change before any code is written - restate it, map files by layer, assess risk, stage the work, and close with a GO or NO-GO verdict."
---

Arguments: `<task description or confirmed step 0 prompt>`

# plan

The main session writes the plan for tier standard and large. No implementation code is written until the plan is accepted (see Closing).

## Skills

- Apply the skills on the step 0 `Skills:` line; `flutter-expert`, `flutter-development`, `spec-driven-development` and `source-driven-development` when installed.
- Ground every framework and package decision in the official Flutter, Dart or package documentation, and read the package source where a default matters.
- Never guess an API, and never plan around a deprecated one.

## Restate

- Restate the requirement in your own words.
- Add no feature the step 0 block did not confirm.
- List every ambiguity as an explicit assumption.

## Search first

- Search for the concept, the feature folder and the entities involved before proposing anything new.
- When the answer needs a sweep across many files, ask the `explorer` for a `file:line` map instead of reading them all here.

## File

Save the plan to `.ai/plans/<task-slug>/plan.md`, one file.

- Tier large adds `code.md` next to it: signatures and the hunks that carry a decision, nothing else.
- `code.md` never holds full test bodies; they are written test first during implementation.

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

## Verdict

Self-gate before closing. The verdict is NO-GO when any criterion below holds. Doubt counts as holding.

- An open item has no owner, or an assumption would change the result if it were wrong.
- The backend response shape is not confirmed by a contract or by existing code.
- The change touches persisted-data schema or dependency registration order without a decision from the user.
- A design or approach choice needs a user decision.
- The scope grew beyond the step 0 block.
- A step 0 decision the plan depends on is missing.

Display the whole plan in chat as plain text, ending with:

```
Verdict: GO | NO-GO
Reason: <the criterion that failed, or "all criteria pass">
```

## Closing

End every plan with:

```
AWAITING CONFIRMATION: proceed with this plan? (yes / change: ... / other approach: ...)
```

- Write no implementation code before an affirmative answer.
- Tier large: after the yes, run the `reviewer` on the plan. Fix its BLOCKER and HIGH findings in the plan before any code; a change in scope goes back to the user.

### QA identifiers

- The plan's architecture or design section lists every QA-relevant widget with its identifier up front. Implementation takes names from that list.
