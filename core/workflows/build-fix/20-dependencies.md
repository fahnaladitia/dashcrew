---
target: workflow/build-fix
order: 20
rules:
  - build.no-version-bump
  - build.local-packages-first
---
## Dependency conflicts

- Never bump a version in the dependency manifest without the user's approval.
- Read the conflicting constraints before proposing anything.
- Version-solving failures usually start in local path packages and vendored packages, not in registry packages. Check those first.
{{#project.local_packages}}
- Local package: `{{.}}`
{{/project.local_packages}}
