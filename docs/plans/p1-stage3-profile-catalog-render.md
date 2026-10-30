# P1 stage 3: profile, catalog, renderer

Tier: standard (three new modules). Scope: library code + unit tests only. No CLI command, no adapter, no file output yet.

## Profile (`.ai/project.yaml`, schema 1)

```yaml
toolkit: 1                       # schema version, required
agents: [claude-code]            # claude-code | codex
language:                        # optional, default en for every key
  dartdoc: en
  pr: en
  chat: en
stack:                           # family -> option, or {use: option, <params>}
  state: bloc
  di: get_it
  router: go_router
  network: dio
  storage: none
  test: manual-fakes
  analytics: { use: posthog, facade: Analytics.instance, catalog: docs/analytics.md }
  tracker: { use: github }
  qa: semantics-identifier
  design: none
project:
  flavors: { dev: lib/main_dev.dart }
  local_packages: []
  base_branches: [main]
  branch_prefix: { feat: feat/, fix: fix/, hotfix: hotfix/ }
  build_wrapper: null
policy:
  agent_commit: false
  formatter: false
  dartdoc_max_lines: 3
  tier_gates: true
  terse_prompts: false
paths:                           # defaults under .ai/
  plans: .ai/plans
  audits: .ai/audits
  pr: .ai/pr
  memory: .ai/memory
```

Validation: unknown top-level keys are errors; `stack` options must exist in the catalog; errors carry the YAML path (`stack.state`).

## Catalog

```
core/<area>/<name>.md                fragment
packs/<family>/<option>/pack.yaml    id, description, params (name -> default)
packs/<family>/<option>/*.md         fragments
```

Fragment front matter:

```yaml
---
target: agents_md | skill/<name> | role/<name> | workflow/<name>
order: 100                 # sort key inside a target
rules: [git.no-agent-commit, git.no-ai-attribution]   # rule ids this fragment covers
when: { policy.agent_commit: false }                   # optional, all must match; list value = any of
---
mustache body
```

## Renderer

1. Core fragments + fragments of the packs selected by `stack`.
2. Drop fragments whose `when` does not match the profile.
3. Group by `target`, sort by `order` then path.
4. Render each body with mustache (strict: a missing variable is an error; no HTML escaping). Context = profile map + `pack.<family>` = selected pack params merged over defaults.
5. Output: `RenderedTarget(target, content, rules)`.

## Tests (written first)

- profile: minimal file gets defaults; unknown top-level key fails with path; `stack` shorthand and map form; unknown option fails with path.
- catalog: fragment front matter parsed; fragment without `target` fails; pack params defaults loaded.
- renderer: unselected pack excluded; `when` filters; order respected; variables substituted; missing variable throws; rule ids collected per target.
