---
target: agents_md
order: 85
when: { build.has_flavors: true }
rules: [build.flavor-run-commands, build.wrapper-for-real-builds]
---
## Build and run

{{#build.flavors}}
- `{{name}}`: `flutter run --flavor {{name}} -t {{entrypoint}}`
{{/build.flavors}}
{{#project.build_wrapper}}
- Real builds go through `{{.}}`, which injects the required defines; a bare `flutter build` may miss them.
{{/project.build_wrapper}}
