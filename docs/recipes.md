# Profile recipes

Complete `.ai/project.yaml` files to start from. Copy the one closest to your project, change what differs, and run `dashcrew sync`. Every key is explained in [docs/profile.md](profile.md).

## A Flutter app with bloc, get_it, go_router and two flavors

Use it for a typical Flutter app: Cubits per screen, one get_it container, go_router, and a dev and a prod entrypoint.

```yaml
toolkit: 1
platform: flutter                     # the default; written out for clarity
agents: [claude-code]
stack:
  state: bloc
  di: { use: get_it, container_file: lib/di/injection.dart }  # a pack param set in the map form
  router: go_router
  network: dio
  test: mocktail
  qa: semantics-identifier            # stable QA identifiers on widgets
project:
  flavors:                            # flavor name -> entrypoint
    dev: lib/main_dev.dart
    prod: lib/main_prod.dart
  base_branches: [main, develop]      # proposed bases for a new branch
policy:
  formatter: true                     # agents may run `dart format` on the files they changed
```

## A Python service on `platform: other`

Use it for a stack dashcrew has no platform for: you name the commands, and you get the whole core (chain, skills, guard, review, PR template) without stack-specific rules.

```yaml
toolkit: 1
platform: other
platform_config:
  detect: [pyproject.toml]            # the file that identifies the stack
  commands:
    analyze: ruff check               # run on the changed paths
    test: pytest
    test_scoped: pytest -q            # test files are appended to it
    deps: pip install -e .
    format: ruff format --check
agents: [claude-code, codex]
stack:
  tracker: { use: github, mode: labels }  # shared families work on every platform
```

## A team on Claude Code, Codex and Antigravity with a merge gate

Use it when teammates use different agents, pull requests must pass `dashcrew check` on GitHub, and the agent commits behind an approval the guard enforces.

```yaml
toolkit: 1
agents: [claude-code, codex, antigravity]  # every agent anyone on the team uses
stack:
  state: riverpod
  di: riverpod
  router: go_router
  network: dio
policy:
  agent_commit: ask                   # the agent runs `git commit`; you approve each call
ci:
  github: true                        # writes .github/workflows/dashcrew.yml
  dashcrew_ref: v0.8.0                # the release CI installs; `ask` needs 0.8.0 or newer
```

Every teammate installs dashcrew 0.8.0 or newer before this profile's first sync is committed. Codex hooks cannot ask for approval, so Codex users print the commit command instead. The full team setup is in [docs/team.md](team.md).

## A Go service

Use it for an HTTP service in Go: Go rules, backend architecture and security rules, and the backend guard rules.

```yaml
toolkit: 1
platform: go                          # init picks it for a go.mod
agents: [claude-code]
stack:
  http: chi                           # nethttp, chi, gin, echo, fiber, none
  db: pgx                             # pgx, sqlc, gorm, none
project:
  base_branches: [main]
```

To rename layers or rewrite the Go rules for your code, eject the platform with `dashcrew platform new go`: it copies the built-in `go` platform into `.ai/platforms/go/`, which your team then owns (see [Platforms](profile.md#platforms)).
