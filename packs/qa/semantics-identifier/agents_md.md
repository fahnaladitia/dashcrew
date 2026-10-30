---
target: agents_md
order: 370
rules: [qa.tag-on-slice, qa.semantics-key-and-identifier, qa.relevant-widgets, qa.naming, qa.no-dynamic-values, qa.no-remove-rename-unchecked, workflow.polish.qa-identifiers, pr.section.qa-identifiers]
---
## QA identifiers

- When slicing or redesigning a screen, tag every QA-relevant widget: anything a test taps, types into or asserts on (buttons, inputs, toggles, checkboxes, list items, tabs, nav actions, dialogs, sheets, key status or amount text). Skip decorative and layout-only widgets.
- Wrap with `Semantics(key: ValueKey(id), identifier: id)`, the same string in both. `identifier` reaches native tools; `key` reaches `find.byKey`.
- Names are snake_case, screen-prefixed, semantic and stable across builds: `{{pack.qa.example_static_id}}`.
- No hash codes, indexes or timestamps. Repeated items key off a stable domain id: `{{pack.qa.example_id}}`.
- Never remove or rename an identifier without checking the automation suites. A missing one is high severity until proven unreferenced.
- The step 0 prompt lists the identifiers to add. The PR description's `## QA Identifiers` section lists those added, changed or removed, or "None".
