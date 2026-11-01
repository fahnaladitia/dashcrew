---
target: agents_md
order: 60
rules: [doc.deliverable, doc.same-edit, doc.comments-not-docs-folder, doc.content-contract, doc.max-lines, doc.overflow-to-plan, doc.language, code.no-docs-on-unchanged]
---
## Documentation

- Doc comments{{#platform.doc_comment}} ({{.}}){{/platform.doc_comment}} on the code are part of the task: code without them is not done. Write them in the same edit as the code, never as a cleanup pass. Notes in a docs folder do not count.
- Say what the unit is for and the one trap a caller cannot guess (ordering, side effect, deliberate exclusion). Never restate the logic.
- At most {{policy.dartdoc_max_lines}} lines of doc comment per unit, a hard cap: an over-long doc is a finding, like a missing one. What does not fit (background, rationale) goes in the plan.
- Write them in language `{{language.dartdoc}}` (ISO 639-1 code), with technical terms kept in English.
- Document the public surface you touch. Never add docs or type annotations to code you are not changing. The `documentation` skill lists where doc comments are required and shows the shape.
