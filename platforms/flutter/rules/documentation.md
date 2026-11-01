---
target: agents_md
order: 60
rules: [doc.deliverable, doc.same-edit, doc.dartdoc-not-docs-folder, doc.content-contract, doc.max-lines, doc.overflow-to-plan, doc.language, code.no-docs-on-unchanged]
---
## Documentation

- Dartdoc (`///`) on the code is part of the task: code without it is not done. Write it in the same edit as the code, never as a cleanup pass. Notes in a docs folder do not count.
- Say what the unit is for and the one trap a caller cannot guess (ordering, side effect, deliberate exclusion). Never restate the logic.
- At most {{policy.dartdoc_max_lines}} lines of `///` per unit, a hard cap: an over-long doc is a finding, like a missing one. What does not fit (background, rationale) goes in the plan.
- Write it in language `{{language.dartdoc}}` (ISO 639-1 code), with technical terms kept in English.
- Document the public surface you touch. Never add docs or type annotations to code you are not changing. The `documentation` skill lists where Dartdoc is required and shows the shape.
