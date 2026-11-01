---
target: skill/documentation
rules: [doc.shape, doc.link-symbols, doc.delete-padding, doc.required.public-types, doc.required.changed-members, doc.required.data-layer, doc.required.navigation-sequencing, doc.required.private-contract, doc.not-required]
---
# Dartdoc

Dartdoc ships in the same edit as the code, at most {{policy.dartdoc_max_lines}} lines of `///`, in language `{{language.dartdoc}}` (ISO 639-1 code) with technical terms in English.

## Required on the units you touch

- Public classes, mixins, enums and extensions.
- Public methods and fields whose behavior or contract changed.
- State holder classes, and every event or state you add.
- Repository methods, mappers, and model fields with a non-obvious source or default.
- Navigation entry points, and anything a caller must call in a specific order.
- Private members that carry a non-obvious contract: ordering, navigation stack or lifecycle side effects, early returns a caller must know about.

## Not required on

- Private members whose names already say everything.
- One-line copy, asset or constant changes.
- Generated files.
- `build()` overrides and mechanical `dispose()` bodies.
- Code you are not changing.

## Shape

1. First line: what the unit is for.
2. Remaining lines, only if needed: the one trap a caller cannot guess (ordering, side effect, deliberate exclusion). Nothing else.

Link related symbols with `[SymbolName]`. If the doc adds nothing beyond the member name, delete it instead of padding it.

Good, a contract in two lines:

```dart
/// Subscribes to [OrderUpdatesBloc] for live status changes.
/// Idempotent: later calls do nothing while already subscribed.
```

Bad, restates the logic line by line:

```dart
/// Toggle the item in the list.
/// If the item is already in the list, remove it.
/// If the item is not in the list, add it.
```
