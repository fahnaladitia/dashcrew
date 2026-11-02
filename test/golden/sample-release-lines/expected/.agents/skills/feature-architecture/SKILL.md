---
name: feature-architecture
description: "Decide a feature's structure before implementation - component split, state boundaries, rebuild scope, DI placement, QA identifiers, data flow and disposal. Use when: For a new screen, state holder or feature folder, or a change across two or more layers."
---

# Feature architecture

Decide the structure before implementation and write it into the plan's `Architecture` section. The decisions bind the implementation: a change to one later is raised to the user and recorded in the PR notes, never made silently.

## Inputs

- The step 0 criteria: every criterion gets a place in the structure (which state, which component).
- The design spec, when there is one (`design-fidelity`): follow its states, components and identifiers unless a search shows a clash, and carry its `Not in design` list.
- The state-management, DI and folder conventions in AGENTS.md and the selected pack skills.

## Decisions

1. Component split: split a component only when it has its own rebuild boundary, is reused, or its parent's render method passes 100 lines. Never split for tidiness. Components stay local to the feature until a second feature uses them.
2. State holder: its type per the selected state pack, and why.
3. Rebuild map: which component may re-render when which state field changes. Default to selective rebuilds for a large state.
4. DI: where each new dependency is registered, and any registration-order risk.
5. QA identifiers: every QA-relevant element with its identifier, per the convention in AGENTS.md. Search the code and tests to confirm each name is unused.
6. Data flow: from repository to UI, naming where the mapper runs. No transport DTO passes the repository.
7. Disposal: every resource the feature creates that must be released, or "none".

## Output

```text
Architecture
Components: <file tree>
State:      <holder type> - <reason>
Rebuild:    <boundaries>
DI:         <registrations and order>
Test ids:   <list>
Data flow:  <chain, mapper position>
Dispose:    <list | none>
Structural risks:
- <what can break later>
```

## In Flutter

- Components are widget classes that can be `const`, never `_buildXxx()` methods; the render method is `build`.
- Rebuild boundaries use the state pack's selective tools (selectors, rebuild conditions) instead of rebuilding the screen.
- The dispose list names every controller, focus node, stream subscription, timer, ticker and state holder created by hand.
