# P10 plan: a second platform, Go, as a level 2 starter

Tier: large (a new command, embedded starter content, a platform.yaml key compiled into the guard policy, new guard rules, a guard minimum that depends on the policy). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P10. Builds on P4 S0 (platform levels) and P9 (`go.work` link).

## Goal

A team adopting Go gets the same chain, guard and review discipline it has on Flutter, without dashcrew deciding the team's Go conventions. dashcrew ships a starter: `dashcrew platform new go` writes `.ai/platforms/go/` into the team's repo, and the team edits it to match its own code. Go becomes built-in (level 3) only later, when a starter that several teams kept unchanged proves generic.

Backend work also brings risks Flutter does not have: migrations against a shared database, destructive SQL, deploys. The guard learns to block those, through a platform key any platform can use.

## What exists

- Level 1 `platform: other` with `platform_config` (commands, and `link` since P9); level 2 project platforms in `.ai/platforms/<name>/` with `platform.yaml`, rules, skills, workflows and `packs/<family>/<option>/`; Flutter is the only built-in platform.
- The `sample-project-platform` golden holds a minimal Go project platform (one rule, one skill).
- `dashcrew link` writes `go.work` (P9).
- `LoadPolicy` ignores keys it does not know. A dashcrew 0.1.x guard reading a policy with new keys enforces the old rules only, silently.

## Design

### The starter

```
dashcrew platform new go [--http nethttp|chi|gin|echo|fiber] [--db pgx|sqlc|gorm] [-p <project>]
```

Writes `.ai/platforms/go/` from content embedded in the binary under `starters/go/` (not `platforms/`, so it is never built-in):

- `platform.yaml`: `detect: [go.mod]`; commands `go vet ./...`, `go test ./...`, scoped `go test -run`, `go mod download`, format `gofmt -l` (check only, never rewrite); `link: go.work`; `doc_comment: Go doc comments`; `guard:` (below).
- Rules, same shape as Flutter's:
  - layers `handler -> service -> repository -> store`, names to be replaced with the team's;
  - `context.Context` first and passed down; no `context.Background()` below `main`;
  - errors wrapped with `%w`, never swallowed, never logged and returned both;
  - goroutines owned and stopped, with `errgroup` or a done channel (the counterpart of Flutter's dispose rule);
  - table-driven tests with hand-written fakes behind small interfaces;
  - doc comments on exported identifiers.
- `review: true` skills: SQL and query safety, auth and permission checks on handlers, secrets in config, migration reversibility.
- Packs, chosen in the profile `stack:`:
  - `http`: `nethttp`, `chi`, `gin`, `echo`, `fiber`, `none`;
  - `db`: `pgx`, `sqlc`, `gorm`, `none`.
- A header line in every starter file: "Starter from dashcrew <version>; edit it to match this repository."

The command refuses when `.ai/platforms/go/` exists (no `--force`: the team owns the folder after the first write). `--http` and `--db` only set the profile's `stack:` choices; every pack is written so the team can switch later. When `.ai/project.yaml` exists it sets `platform: go` and the chosen `stack:`, and prints the `dashcrew sync` to run. `dashcrew init` on a repo with `go.mod` and no `.ai/platforms/go/` offers the starter.

### Backend guard rules, generic

A platform (built-in, project, or `platform_config` at level 1) may declare:

```yaml
guard:
  commands:            # program and leading subcommands; options are skipped
    - [terraform, apply]
    - [terraform, destroy]
    - [kubectl, apply]
    - [kubectl, delete]
    - [helm, install]
    - [helm, upgrade]
    - [helm, uninstall]
  sql:
    clients: [psql, mysql, mariadb, sqlite3]
    statements: [DROP, TRUNCATE]
  migrations:
    tools: [goose, migrate, atlas]
    local_hosts: [localhost, 127.0.0.1, "::1"]
```

- `commands`: blocks a command whose program and first non-option words match an entry, through every wrapper and nested shell the guard already unwraps.
- `sql`: blocks a client command whose SQL text, given as an argument, a here-string or a heredoc, holds one of the statements as a word (case-insensitive).
- `migrations`: blocks a migration tool run whose connection string (an argument, `-database`/`--url`, or a `VAR=` assignment before it) names a host outside `local_hosts`. A run whose connection string the guard cannot see (read from the environment or a file) is allowed. The README says so; it matches how the guard treats other blind spots.
- `sync` compiles these into `.ai/guard.json` as `block_commands`, `block_sql` and `block_migrations`. Without a `guard:` key nothing changes: Flutter goldens stay identical.
- **Guard minimum per policy:** when any of the three keys is present, `sync` writes `guard_min_version: 0.2.0` into the lock and the shim, so a 0.1.x dashcrew, which would ignore them, is refused (fail closed) instead of half-enforcing.

### Docs

- README: Platforms section gets the starter and the `guard:` key; a Go quick start.
- Wiki page "Go projects".

## Stages

### G10-0: `guard:` in platform.yaml and the policy

- Catalog: parse and validate `guard:` for built-in and project platforms and `platform_config`; strict keys.
- `CompilePolicy` writes the three keys; `LoadPolicy` reads them (optional, typed).
- Sync: guard minimum 0.2.0 when present; shim and lock follow it.
- Tests: compile and load round trip; a policy without the keys is byte-identical to today; the minimum moves only with the keys.

### G10-1: guard rules

- `commands`, `sql`, `migrations` matchers in `internal/guard`, reusing the unwrapping of nested shells, wrappers and heredocs.
- Corpus: existing outcomes unchanged; new cases per rule, including wrappers (`sudo`, `env`, `bash -c`), heredocs into `psql`, DSNs with and without a host, `@localhost`, IPv6, and the allowed blind spot.

### G10-2: the starter

- `starters/go/` embedded; `internal/starter` writes it; `dashcrew platform new go` with `--http`, `--db`.
- `init` offers it on `go.mod`.
- Goldens: a `sample-go` fixture generated from the starter, synced with each http and db choice once.

### G10-3: Go content

- Rules, review skills and packs as listed above, each pack with its own rule lines (for example gin: bind and validate request bodies, return errors through one middleware; gorm: no `AutoMigrate` outside local, scopes for tenancy; pgx: pool from context, `pgx.ErrNoRows` mapped once in the repository).
- Content is reviewed against the official documentation of each library before it is written.

### G10-4: live

- A sample Go service (handler -> service -> repository) built with the starter, with chi and pgx, tests with fakes, no running database.
- One task through the chain with Claude Code: plan, tdd, review, verify, PR description, commit command.
- Guard live through the shim: a `terraform apply`, a `psql -c "DROP TABLE x"`, a `goose up` against a remote DSN and the same against `localhost`.

## How we check

- G10-0: policy round trip tests; Flutter goldens and corpus unchanged; lock and shim minimum transcripts.
- G10-1: new corpus cases; differential over the existing corpus identical.
- G10-2: starter files byte-for-byte from the embedded tree; refusal on an existing folder; `init` transcript on a Go repo.
- G10-3: render test that every Go pack renders and AGENTS.md stays under the agent limits.
- G10-4: live run notes in this plan, with token counts per role like the P4 run.

## Not doing

- A built-in Go platform (level 3).
- Go packs beyond http and db (migrate, di, log come when a team uses them).
- Reading connection strings from environment variables or files.
- Linter setup (`golangci-lint`): only used when the repo already configures one, as a command the team adds.

## Decisions (all confirmed)

1. Shape: a level 2 starter written by `dashcrew platform new go`.
2. Guard: all three backend rules (non-local migrations, DROP/TRUNCATE through SQL clients, deploy and infra apply), through a generic `guard:` platform key.
3. Packs: http (`nethttp`, `chi`, `gin`, `echo`, `fiber`) and db (`pgx`, `sqlc`, `gorm`).
4. Live test on a sample service built for it.

5. A policy with backend rules raises the guard minimum to 0.2.0 for that project only; Flutter projects keep accepting 0.1.x.
6. `gofmt` in the chain is check only (`gofmt -l`); the no-formatter rule stays.

## Progress

- [x] G10-0 guard key and policy: `profile.PlatformGuard`, catalog and `platform_config` parse it, policy keys `block_commands`/`block_sql`/`block_migrations`, `Policy.Minimum()` drives shim and lock.
- [x] G10-1 guard rules: `internal/guard/backend.go`; corpus unchanged; `backend_test.go`.
- [x] G10-2 starter: `starters/go/` embedded (`StartersFS`), `internal/starter`, `dashcrew platform new`, `init` tip, golden `sample-go` written from the starter.
- [x] G10-3 Go content: rules (go, architecture, security, testing), `backend-security`, `data-layer-tdd`, `feature-architecture`, packs http and db; every http x db pair syncs for three agents under the Antigravity limit.
- [ ] G10-4 live
