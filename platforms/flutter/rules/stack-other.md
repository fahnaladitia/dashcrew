---
target: agents_md
order: 390
when: { has_stack_other: true }
rules: [stack.other-follow-official-docs, stack.other-follow-codebase, stack.other-no-second-library]
---
## Libraries without a dashcrew pack

{{#stack_other}}
- {{family}}: `{{name}}`
{{/stack_other}}

- Follow the official documentation of the version locked in `pubspec.lock`, and the patterns this codebase already uses. Read existing usages before adding a new pattern.
- Never add a second library for the same concern.
