# Catalog format

Neutral sources live in `core/` and `packs/`. Every `.md` file is a fragment: YAML front matter plus a Mustache body. The renderer joins fragments per `target`, sorted by `order` then path.

## Layout

```
core/rules/<area>.md                       target: agents_md
core/workflows/<name>/meta.yaml            description, argument_hint
core/workflows/<name>/*.md                 target: workflow/<name>
core/roles/<name>/role.yaml                description, capability, model, effort
core/roles/<name>/*.md                     target: role/<name>
core/skills/<name>/meta.yaml               description, when_to_use
core/skills/<name>/*.md                    target: skill/<name>
packs/<family>/<option>/pack.yaml          description, params (name -> default)
packs/<family>/<option>/*.md               any target; rendered only when selected
```

Pack families: `state`, `di`, `router`, `network`, `storage`, `test`, `analytics`, `tracker`, `qa`, `design`.

## Front matter

```yaml
---
target: agents_md           # agents_md | workflow/<name> | role/<name> | skill/<name>
order: 100                  # optional, default 100
rules: [git.no-agent-commit] # rule ids this fragment states
when: { policy.agent_commit: false }  # optional; all keys must match, a list means any of
---
```

## role.yaml

```yaml
description: One sentence, used by the agent to pick the role.
capability: read            # read | write-plans | write-code
model: high                 # high | standard (adapter maps to a concrete model)
effort: high                # low | medium | high
```

`read` roles never edit files; `write-plans` roles may write only under `{{paths.plans}}`; `write-code` roles edit source and tests.

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
- Generic examples only (`orders`, `profile_page`, `order_item_${order.id}`); never names, screens, domains or paths from any real project.
- `agents_md` holds what an agent must always know. Procedures, checklists and long tables go to a workflow, role or skill and are linked by name.
