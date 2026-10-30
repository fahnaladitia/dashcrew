---
target: agents_md
order: 310
rules: [di.register-in-container, di.get-it-not-in-pubspec]
---
## Dependency injection: get_it

- Register every new dependency in `{{pack.di.container_file}}`. A new dependency left unregistered is a high-severity review finding.
{{#pack.di.transitive_only}}
- `get_it` reaches the app transitively and stays out of `pubspec.yaml`. Never add it as a direct dependency.
{{/pack.di.transitive_only}}
{{^pack.di.transitive_only}}
- Never add, remove or re-version `get_it` in `pubspec.yaml` without asking.
{{/pack.di.transitive_only}}
