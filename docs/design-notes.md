# Design notes

Why dashcrew is built the way it is. The full history, one plan per change, is in [docs/plans](plans/); [architecture.md](architecture.md) maps the code.

## Why skill-first

Before 0.1.0, a task ran through role subagents: personas for planning, building, reviewing and a tech-lead gate. A measured run of that flow on a small sample app took 16 role runs and about 1.25M output tokens. Each role started with an empty context and re-read the codebase, and the handoff files existed only to coordinate between those contexts. The run ended in NO-GO with no blocking review finding: three roles' rules contradicted each other. Roles were also the least portable output, since not every agent can limit a subagent to a path or a tool list.

What actually produced quality survived as skills and workflow steps: reading package source before use, an independent review of the diff, a test-first data layer, and scoped verification with real output. So one main session plans and builds, skills are picked once at step 0 in one visible list, and only two subagents remain where a separate context earns its cost: `reviewer` and `explorer`, both read-only.

## Why no server and no database

- **Git is the store.** Rules, knowledge pages, specs and plans are Markdown in git: reviewable, diffable, and synced by the tools a team already uses. Approval is a merged pull request, not a button.
- **Indexes are derived.** `knowledge serve` builds its BM25 index in memory and rebuilds it when the clone's HEAD moves. No embeddings, no vector store; nothing to back up or migrate.
- **No hosting.** A server would add auth, hosting and data to protect. The tracker board and GitHub already show who is on what.
- **One static binary.** The catalog is built in, so a pinned version renders the same output on every machine, and CI installs it with one command.

## The guard: fail closed, fail open, and its limits

The guard runs before every agent tool call, so it must never be the reason work stops for no reason, and never silently disappear.

- **Fail closed on the binary.** The committed shim refuses every call when `dashcrew` is missing or older than the lock's `guard_min_version`. A teammate without dashcrew cannot run an unguarded agent by accident, and a policy that needs a newer guard (backend rules, a push rule, `ask`) raises that minimum, so an older guard never reads it half.
- **Fail open on input.** A hook payload the guard cannot read (not JSON, not UTF-8, a field of the wrong type), and a missing or broken policy, let the call through. A changed agent hook format must not stop a whole team. Each such payload is logged, and `doctor` warns (`guard-log`, `guard-policy`), so the gap is visible. A tool name the guard does not know is logged too, and its `command`, when it has one, is still checked as a shell command.
- **Ask only an agent it knows.** With `agent_commit: ask`, the guard answers ask instead of allow, and the agent prompts the user. An agent that cannot ask treats that answer as a failed hook and runs the call, so the guard answers ask only to an agent it identifies for sure, and blocks for every other.
- **Limits.** The guard prevents mistakes, not a determined attacker. It reads nested shells, wrappers and heredocs up to 8 levels, but it does not block a first word that is a `$` expansion, `source` or `.`, `fish -c`, `stdbuf`, `doas`, or deeper nesting: blocking those would block legitimate scripts. The list is in [guard.md](guard.md). It is a seatbelt, not a sandbox; run agents with the permissions you would give a new teammate.

## Smaller choices

- **Generated files are never edited by hand.** Changes go to their source, so `sync` is repeatable and `doctor` can report drift.
- **Output is pinned byte for byte** by goldens and CLI transcripts, which is how the Go engine proved it matched the Dart one.
- **Agent-neutral sources.** `core/` never holds agent-specific fields; adapters add them per agent.
- **Git history stays with people by default.** dashcrew prints commit commands and checks them. A team can let the agent commit and push its task branch on the user's answer (`agent_commit`, `agent_push`), or behind a prompt the guard enforces (`ask`). History rewrites, force pushes, merges and approvals stay the user's in every policy.
