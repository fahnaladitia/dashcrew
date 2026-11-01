---
name: design-fidelity
description: "Implement a UI from the design frame the user gave, exactly as drawn, through a design spec in the plan, and record every gap."
when_to_use: "When a task comes with a design link, or slices or redesigns a screen."
---

# Design fidelity

You guard the design; you do not create it. The only visual source of truth is the frame the user gave.

## Source

- Never search the design file for frames beyond the links the user gave, and never open the design tool without a link from the user.
- Read the success criteria first: they say which states need a frame.
- Search the repo for a match for every token and component before writing the spec. The selected design pack names the tool-specific steps.

## Mapping

- Tokens: map colors, text styles, spacing and radius to the ones the project already has. Use a raw value only when no token exists, and mark it `NEW TOKEN`.
- Components: map each design component to an existing one and the parameters used. A new component is a finding to raise, not the default.
- States: one table per screen of loading, empty, error, success, disabled and partial -> frame id -> component.

## Copy, states and assets

- Take copy exactly from the design, case and punctuation included; never invent it. Copy the server provides is marked `SERVER-DRIVEN` and never hardcoded.
- A state with no frame (loading, empty, error) follows the closest comparable merged component exactly and goes on the `Not in design` list; search the codebase for the same label first. Never add states, copy, icons or animations the frame does not show.
- Assets: name, frame id, position and root size. Download them through the design tool's export path, compare them against the frame screenshot (exported glyphs can be wrong), and leave no temporary design-tool URL in code.
- Test identifiers: propose one per QA-relevant element, per the convention in AGENTS.md, and check each is unused.

## Design spec

Write it into the plan's `Design` section before UI code:

```text
Frames:      <frame ids + names>
Tokens:      <design -> project, NEW TOKEN rows>
Components:  <design -> project component | NEW>
States:      <state -> frame id -> component>
Copy:        <verbatim list, SERVER-DRIVEN marks>
Assets:      <frame id, position, size>
Test ids:    <proposals>

Not in design:
- <state, copy or variant without a frame, and the merged component it follows>
```

## Gaps

- Something that looks like a design mistake goes under `Open questions` for the designer; implement the design as drawn.
- The `Not in design` list travels with the task: the plan holds it, the implementation follows it, the `reviewer` checks it, and it ends in the PR description notes section.

## Review checklist

- Copy, tokens and components match the spec; every `NEW TOKEN` and `NEW` component was raised.
- No state, copy, icon or animation that no frame shows, except `Not in design` items.
- No hardcoded `SERVER-DRIVEN` text and no temporary design-tool URL.
- Every `Not in design` item is recorded for the PR notes.
