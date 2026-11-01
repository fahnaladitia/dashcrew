---
target: skill/figma-design
order: 100
rules: [figma.skill-before-fetch, figma.read-order, figma.frame-wins, figma.follow-frame-over-neighbour, figma.map-before-new, figma.optional-param-variation, figma.unframed-states-are-questions, figma.node-level-diff]
---
# Figma

## Reading

- Load the design-to-code skill of your Figma tool before the first design-context fetch.
- For each linked frame, take a screenshot first, then fetch its structured design context.
- Request bulk node metadata sparingly and in small batches; it is expensive and can return a whole section.
- Read the design variables for color, typography, spacing and radius tokens.

## Source of truth

- Translate the frames the user gave; never redesign them.
- On conflict between a frame and a tracker or task description, the frame wins. Record the conflict in the spec.
- When the frame differs from a neighbouring screen, follow the frame as drawn and note the difference; ask only when the frame is ambiguous.

## Mapping

- Map every color, text style, spacing and radius to an existing token, and every element to an existing component, before proposing anything new.
- A per-screen variation of a shared component becomes an optional parameter whose default keeps today's look.
- List states, empty and error views, and copy that have no frame as open questions. Never invent them.

## Node-level diff before done

- Before marking a frame done, compare node by node: text and copy, typography (size, weight, line height), color, spacing, radius, borders, icons, and every state.
- "The element exists" is not "the element matches". Record each mismatch with the node name and the expected and actual value.
