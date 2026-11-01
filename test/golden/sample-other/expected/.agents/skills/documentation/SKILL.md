---
name: documentation
description: "Where doc comments are required, where they are not, and the shape they take. Use when: When writing or reviewing a documented unit."
---

# Doc comments

Doc comments ship in the same edit as the code, at most 3 lines, in language `en` (ISO 639-1 code) with technical terms in English.

## Required on the units you touch

- Public types: classes, interfaces, enums and their equivalents.
- Public methods and fields whose behavior or contract changed.
- State holder classes, and every event or state you add.
- Repository methods, mappers, and model fields with a non-obvious source or default.
- Navigation entry points, and anything a caller must call in a specific order.
- Private members that carry a non-obvious contract: ordering, navigation stack or lifecycle side effects, early returns a caller must know about.

## Not required on

- Private members whose names already say everything.
- One-line copy, asset or constant changes.
- Generated files.
- Render methods and mechanical cleanup bodies.
- Code you are not changing.

## Shape

1. First line: what the unit is for.
2. Remaining lines, only if needed: the one trap a caller cannot guess (ordering, side effect, deliberate exclusion). Nothing else.

Link related symbols with the language's link syntax. If the doc adds nothing beyond the member name, delete it instead of padding it.

Good, a contract in two lines:

```text
Subscribes to OrderUpdates for live status changes.
Idempotent: later calls do nothing while already subscribed.
```

Bad, restates the logic line by line:

```text
Toggle the item in the list.
If the item is already in the list, remove it.
If the item is not in the list, add it.
```
