# Catalog format

Neutral sources live in `core/`, `packs/` and `platforms/`. Every `.md` file is a fragment: YAML front matter plus a Mustache body. The renderer joins fragments per `target`, sorted by `order` then path.

`core/` is platform-agnostic. Everything that belongs to one stack (its commands, lifecycle rules, doc comment convention, build tools, pack families) lives in `platforms/<platform>/`, and only the platform the profile selects renders.

## Layout

```
core/rules/<area>.md                       target: agents_md
core/pr/*.md                               target: pr_template (.github/pull_request_template.md)
core/workflows/<name>/meta.yaml            description, argument_hint
core/workflows/<name>/*.md                 target: workflow/<name>
core/roles/<name>/role.yaml                description, capability, model, effort
core/roles/<name>/*.md                     target: role/<name>
core/skills/<name>/meta.yaml               description, when_to_use, review
core/skills/<name>/*.md                    target: skill/<name>
packs/<family>/<option>/pack.yaml          description, params (name -> default)
packs/<family>/<option>/*.md               any target; rendered only when selected
platforms/<platform>/platform.yaml         detect, commands, link, doc_comment, description, mcp_servers
platforms/<platform>/rules|skills|workflows/  fragments and meta.yaml, same front matter as core
platforms/<platform>/packs/<family>/<option>/ the platform's pack families, same format as packs/
```

Shared pack families in `packs/`: `tracker`, `design`. Families of the `flutter` platform in `platforms/flutter/packs/`: `state`, `di`, `router`, `network`, `storage`, `test`, `analytics`, `qa`.

## Front matter

```yaml
---
target: agents_md           # agents_md | workflow/<name> | role/<name> | skill/<name> | pr_template
order: 100                  # optional, default 100
rules: [git.no-agent-commit] # rule ids this fragment states
when: { policy.agent_commit: false }  # optional; all keys must match, a list means any of
---
```

## Skill meta.yaml

```yaml
description: One sentence: what the skill is.
when_to_use: When the agent should pick it at step 0.   # optional; the skill index falls back to description
review: true                # optional, default false: the reviewer applies the skill's checklist when it is active
```

A pack skill uses the same keys in `skills/<name>.yaml`. Every skill that renders gets one line in the AGENTS.md skill index (`skill_index`), sorted by name.

## role.yaml

```yaml
description: One sentence, used by the agent to pick the role.
capability: read            # read | write-plans | write-code
model: high                 # high | standard (adapter maps to a concrete model)
effort: high                # low | medium | high
```

`read` roles never edit files; `write-plans` roles may write only under `{{paths.plans}}`; `write-code` roles edit source and tests. The catalog ships two `read` roles, `reviewer` and `explorer`, both in `core/roles/`. Roles are defined only there: packs and platforms add skills and workflow sections, never new roles. There is no `orchestrate` or `checkpoint` workflow; the chain in AGENTS.md runs the workflows.

## Platforms

The profile's `platform` selects one platform: a built-in one (`flutter`), a project platform, or `other`. A profile without the key reads as `flutter`.

```yaml
description: Flutter apps and packages written in Dart.   # optional
detect: [pubspec.yaml]          # required: files whose presence identifies the platform
commands:                       # required: analyze, test, test_scoped, deps; format is optional
  analyze: flutter analyze
  test: flutter test
  test_scoped: flutter test --reporter expanded
  deps: flutter pub get
  format: dart format
link: pubspec_overrides.yaml    # optional: how repositories link a local package
doc_comment: Dartdoc            # optional: the doc comment convention
mcp_servers:                    # optional: MCP servers sync adds to each agent's config
  dart-mcp-server: { command: dart, args: [mcp-server] }
```

- `test_scoped` takes test files appended to it; `test` runs the whole suite; `deps` fetches dependencies (`dashcrew worktree create` runs it when one of the `detect` files exists).
- A platform fragment at the same path as a core fragment replaces it: `platforms/flutter/rules/testing.md` replaces `core/rules/testing.md`. Use it when generic and platform text interleave line by line; otherwise add a new fragment whose `order` places it in the rendered sequence.
- A platform `meta.yaml` for a skill or workflow that core also defines replaces the core description.
- Stack families resolve inside the selected platform first, then in the shared `packs/`; a project pack from `.ai/packs/` wins over both.
- `init` matches the `detect` files of every built-in and project platform. A repo that matches none is offered `other`; one that matches several is asked which.
- The name `other` is reserved. `platform: other` takes the same fields inline, `platform_config: { detect: [...], commands: {...} }`, and renders core only with the shared packs.

### Project platforms

A project keeps its own platforms in `.ai/platforms/<name>/`, in the same format as `platforms/`. `sync`, `doctor` and `init` load them next to the built-in ones; a project platform with a built-in name replaces it whole. Errors point at the project file (`.ai/platforms/go/platform.yaml`).

## Template variables

Templates are strict: an unknown variable is a render error. Use only these.

| Variable | Default |
|---|---|
| `language.dartdoc`, `language.pr`, `language.chat` | `en` |
| `policy.agent_commit` | `false` |
| `policy.formatter` | `false` |
| `policy.dartdoc_max_lines` | `3` |
| `policy.tier_gates` | `true` |
| `policy.terse_prompts` | `false` |
| `paths.plans`, `paths.audits`, `paths.pr`, `paths.memory` | `.ai/plans`, `.ai/audits`, `.ai/pr`, `.ai/memory` |
| `ci.github`, `ci.dashcrew_ref` | `false`, null |
| `project.flavors` (map of flavor name to entrypoint), `project.local_packages` (list) | empty |
| `project.base_branches` (list) | `[main]` |
| `project.branch_prefix.feat` / `.fix` / `.hotfix` | `feat/`, `fix/`, `hotfix/` |
| `project.build_wrapper` | null |
| `build.flavors` (list of `name`, `entrypoint`) | `project.flavors` sorted by name; empty |
| `build.has_flavors` | `true` when `build.flavors` is not empty |
| `stack.<family>.use` | selected option; absent when the family is not selected |
| `stack_other` (list of `family`, `name`) | the families set to `use: other`, sorted by family; empty |
| `has_stack_other` | `true` when `stack_other` is not empty; gate fragments that loop over it |
| `pack.<family>.<param>` | pack param; use only inside that pack's own fragments |
| `platform.name` | the selected platform, or `other` |
| `platform.commands.analyze`, `.test`, `.test_scoped`, `.deps` | the platform's commands |
| `platform.commands.format`, `platform.link`, `platform.doc_comment`, `platform.description` | null when not declared; use them inside a section (`{{#platform.commands.format}}...{{/platform.commands.format}}`) |
| `platform.detect` (list) | the platform's detect files |
| `skill_index` (list of `name`, `when_to_use`, `review`) | every skill that renders and has a description, sorted by name; `when_to_use` falls back to the description |

Every family also accepts `use: other` with a required `name` (the library package name) for a library that has no pack. It selects no pack, so `pack.<family>` is absent; `core/rules/stack-other.md` renders the generic rules.

Inverted sections (`{{^x}}`) over a string value fail in strict mode; write optional text with a positive section only.

Lists: `{{#project.base_branches}}` `{{.}}` `{{/project.base_branches}}`. A section over a key with a default (`{{#project.build_wrapper}}...{{/project.build_wrapper}}`) is safe. A section over `stack.<family>` is not: an unselected family is an error, so gate those fragments with `when` instead.

## Project packs

A project keeps its own packs in `.ai/packs/<family>/<option>/`, in the same format as `packs/` (`pack.yaml`, fragments, `skills/<name>.yaml`). `sync` and `doctor` overlay them on the built-in catalog:

- Same family and option as a built-in pack: the project pack replaces it whole; nothing is merged. Fragments of other packs gated on `stack.<family>.use: <option>` still render, because the option name stays.
- A new family (`architecture/clean`) becomes a valid `stack` key for that project.
- A skill whose `skills/<name>.yaml` came from the replaced pack is skipped with a warning unless the project pack ships it again.
- The option name `other` is reserved.
- Errors point at the project file (`.ai/packs/state/bloc/pack.yaml`).
- `.ai/lock.json` records `packs` as `family/option -> builtin | project`; the `project-packs` doctor check fails on a pack that does not parse and warns on one the profile does not select.

## Writing rules

- English, imperative, short. One obligation per sentence.
- Core never names a platform's tools or conventions: write `{{platform.commands.test_scoped}}`, not a literal command, and put stack-specific text in `platforms/<platform>/`.
- Generic examples only (`orders`, `profile_page`, `order_item_${order.id}`); never names, screens, domains or paths from any real project.
- `agents_md` holds what an agent must always know. Procedures, checklists and long tables go to a workflow or skill and are linked by name; a checklist the `reviewer` should apply goes in a skill with `review: true`.
