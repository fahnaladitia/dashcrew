# P3e plan: one guard per machine

Tier: standard (guard install location, generated shim, `.gitignore` handling, doctor). Before P4.

## Problem

The guard is compiled per project into `.ai/bin/guard`, which is git-ignored. After a clone, no guard rule applies until that person runs `dashcrew sync`, and the hook fails open, so nothing says so. Every new project on the same machine compiles its own copy (about 10 s, 6 MB).

An idea from another agent-agnostic tool: its guard is a subcommand of the CLI on `PATH`, so a fresh clone is guarded as soon as the tool is installed.

## Measured (2026-10-31, macOS, 20 runs after one warm-up, same payload)

| How the hook calls the guard | median | p90 |
|---|---|---|
| AOT executable (`.ai/bin/guard` today) | 12.1 ms | 13.7 ms |
| `dartaotruntime` with an AOT snapshot | 14.1 ms | 15.8 ms |
| `dart pub global run dashcrew` (the `PATH` wrapper pub installs) | 268.7 ms | 302.2 ms |

A subcommand behind pub's wrapper costs about 270 ms on every tool call, far over the 20 ms budget from P1. The guard stays an AOT executable; what changes is where it lives and how a project reaches it.

## Design

- **Global binary.** `dashcrew guard install` compiles the guard once per dashcrew version into `${DASHCREW_HOME:-$HOME/.dashcrew}/guard/<version>/guard`, shared by every project on the machine. `sync` runs it when that file is missing.
- **Committed shim.** `.ai/bin/guard` becomes a generated POSIX `sh` script, tracked in the lock and committed:
  - it `exec`s the global binary of the version in the lock, passing its arguments and stdin;
  - when the binary is missing and `dashcrew` is on `PATH`, it runs `dashcrew guard install` first (one wait of about 2 s on the first tool call on a new machine, measured), then `exec`s;
  - when that is impossible, it prints `[dashcrew guard] not installed on this machine: run dashcrew sync` on stderr and exits 0 (fail-open, as today).
- **`.gitignore`.** The `.ai/bin/` line dashcrew added is removed, since the shim is meant to be committed. Old global binaries are never deleted automatically.
- **Doctor.** `guard-binary` checks the shim and the global binary of the locked version, and still runs a `git push` event through the hook path.
- Hook commands stay as they are (`.ai/bin/guard` with the same arguments), so the three adapters and their goldens change only by the lock.

## Not doing

- Windows: the shim is `sh`; Windows support needs its own shim and is not planned yet.
- Removing old versions from `~/.dashcrew`: a `dashcrew guard prune` can come later.
- Fixing the Antigravity working-directory risk: the hook still calls `./.ai/bin/guard`. A shim on `PATH` would need a `PATH` that GUI apps may not have.

## Stages

- **G1**: `dashcrew guard install` and the versioned global location; `sync` installs when missing. Tests with `DASHCREW_HOME` in a temp dir.
- **G2**: the shim, generated and locked; the `.gitignore` migration; latency of shim plus binary measured against the 20 ms budget.
- **G3**: doctor `guard-binary` for the shim and the global binary; README (install, team onboarding, troubleshooting).

## Decisions needed

1. Location: `~/.dashcrew` with `DASHCREW_HOME` to override (proposed), or the platform cache folder (`~/Library/Caches`, `$XDG_CACHE_HOME`)?
2. A missing binary on the first tool call: install on demand inside the hook (proposed; one wait of about 2 s), or only print the message and fail open?
3. Commit `.ai/bin/guard` (the shim) and drop the `.ai/bin/` ignore line (proposed)?

## Decisions (confirmed)

1. `~/.dashcrew`, overridable with `DASHCREW_HOME`.
2. Install on demand inside the hook.
3. Commit the shim and drop the `.ai/bin/` ignore line.

## Progress

- G1 done, with one change to the design: the global folder is named after the guard source hash (`<home>/guard/<first 16 hex of guard_source>/guard`), not the dashcrew version, because unreleased commits all report 0.0.1 and a changed guard would otherwise reuse a stale binary. `GuardInstaller` in `lib/src/sync/guard_install.dart` (outside `lib/src/guard/`, whose files feed the hash) and `dashcrew guard install [--expect <hash>] [--force]`. `--expect` refuses a dashcrew whose guard source differs from the project's, before compiling, so a teammate on another commit never fills the slot with a different guard. Measured: first install 2.1 s, a second call 0.7 s with no compile; the installed binary blocks a `git push` event.
