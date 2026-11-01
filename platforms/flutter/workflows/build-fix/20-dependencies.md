---
target: workflow/build-fix
order: 20
rules:
  - build.no-version-bump
  - build.local-packages-first
  - build.resolve-package-skill
---
## Dependency conflicts

- Never bump a version in `pubspec.yaml` without the user's approval.
- Read the conflicting constraints before proposing anything.
- Version-solving failures usually start in local path packages and vendored packages, not in pub.dev packages. Check those first.
{{#project.local_packages}}
- Local package: `{{.}}`
{{/project.local_packages}}
- For a `pub get` version conflict, apply `dart-resolve-package-conflicts`.
