---
target: workflow/build-fix
order: 30
rules:
  - build.wrapper-script
  - build.flavors
---
## Real builds

{{#project.build_wrapper}}
- Real builds go through `{{.}}`, which injects the required `--dart-define` values. A plain `flutter build` fails or produces a misconfigured binary.
{{/project.build_wrapper}}
- When the project defines flavors, run and build each flavor with its own entrypoint (`flutter run --flavor <flavor> -t <entrypoint>`); the flavor and entrypoint pairs are listed in AGENTS.md.
- When flavors exist, build Android with the flavor's Gradle task (`assemble<Flavor>Debug`), not the plain `assembleDebug`.
