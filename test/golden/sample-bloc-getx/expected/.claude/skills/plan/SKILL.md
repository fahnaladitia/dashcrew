---
name: plan
description: "Plan a change before any code is written - restate it, map files by layer, assess risk, stage the work, and close with a GO or NO-GO verdict."
argument-hint: "<task description or confirmed step 0 prompt>"
---

# plan

Runs the `flutter-planner` role. No implementation code is written until the plan is accepted (see Closing).

## Skills

- Apply `flutter-expert`, `flutter-development`, `spec-driven-development` and `source-driven-development`.
- Ground every framework and package decision in the official Flutter, Dart or package documentation.
- Never guess an API, and never plan around a deprecated one.

## Restate

- Restate the requirement in your own words.
- Add no feature the prompt did not ask for.
- List every ambiguity as an explicit assumption.

## Files

Save the plan to `docs/plans/<plan-folder>/` as two files:

- `plan.md`: requirements, strategy, architecture, affected files, edge cases, leak analysis.
- `code.md`: the concrete code changes to `lib/` per file, plus a test list (file, test name, setup, main assertion).

- `code.md` never holds full test bodies; the implementing role writes them.
- `flutter-planner` writes `plan.md` and `code.md` itself. The main session never retypes their content.

## Required sections

### Acceptance criteria and boundaries

- Write concrete, checkable acceptance criteria.
- Write explicit boundaries in three lists: Always, Ask first, Never.

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

## Branch

For a new feat, fix or hotfix, end the plan with:

- exactly 3 branch names, kebab-case, naming the area, with the prefix `feature/`, `bugfix/` or `hotfix/`
- a question asking which base branch to fork from; never assume it (common bases: `develop` `main`)
- the output of `git status` when the working tree is dirty

## Verdict

Self-gate before closing. The verdict is NO-GO when any criterion below holds. Doubt counts as holding.

- An open item has no owner, or an assumption would change the result if it were wrong.
- The backend response shape is not confirmed by a contract or by existing code.
- A design or approach choice needs a user decision.
- The scope grew beyond the prompt confirmed at step 0.

Display the whole plan in chat as plain text, ending with:

```
Verdict: GO | NO-GO
Reason: <the criterion that failed, or "all criteria pass">
```

## Closing

When step 0 set the tier to `trivial` or `small` and the verdict is GO, end with:

```
CONTINUE: tier <tier>, verdict GO. Implementation starts without confirmation.
```

In every other case (tier `standard` or `large`, or NO-GO on any tier), end with:

```
AWAITING CONFIRMATION: proceed with this plan? (yes / change: ... / other approach: ...)
```

Write no implementation code before an affirmative answer, except in the CONTINUE case above.

### Dependency injection

- NO-GO when the change reorders existing registrations in `lib/injection_container.dart`.

### Local storage

- NO-GO when the plan changes a `@HiveType` (new type, `typeId`, or field order).

### Analytics

- The plan always has an Analytics section: does the change add, change, remove or not touch a user action, screen or funnel step? List candidate events from `docs/analytics.md` and recommend asking the user.
- Carry the step 0 analytics decision into the plan. Ask again only when step 0 left it open.
- NO-GO when the step 0 analytics decision is missing.

### QA identifiers

- The architecture or plan lists every QA-relevant widget with its identifier up front. Implementation takes names from that list.
