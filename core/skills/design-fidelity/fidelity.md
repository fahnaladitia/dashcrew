---
target: skill/design-fidelity
rules: [design.single-source-frame, design.no-design-tool-without-link, design.map-tokens-components, design.copy-exact, design.no-invented-states, design.assets-no-temp-urls, design.fixes-as-questions, design.gaps-carried-to-pr]
---
# Design fidelity

## Source

- The only visual source of truth is the frame the user gave. Never search the design file for other frames.
- Never open the design tool without a link from the user.

## Mapping

- Map colors, text styles, spacing and components to the ones the project already has. Use a raw value only when no token exists, and mark it `NEW TOKEN`.
- A new component is a finding to raise, not the default.

## Copy, states and assets

- Take copy exactly from the design, case and punctuation included; never invent it. Copy the server provides is marked `SERVER-DRIVEN` and never hardcoded.
- A state with no frame (loading, empty, error) follows the closest comparable merged component exactly and goes on the `Not in design` list. Never add states, copy, icons or animations the frame does not show.
- Download assets through the design tool's export path, compare default variants against the frame screenshot, and leave no temporary design-tool URL in code.

## Gaps

- Something that looks like a design mistake goes under `Open questions` for the designer; implement the design as drawn.
- The `Not in design` list travels with the task: the architect writes it, the engineer uses it, the tech lead checks it, and it ends in the PR description notes section.
