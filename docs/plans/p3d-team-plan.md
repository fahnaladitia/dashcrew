# P3d plan: team readiness

Tier: standard (lock schema field, sync and doctor behavior, README). Before P4.

Goal: a team of several people, each with their own agent (Claude Code, Codex, Antigravity), shares one repository and one generated toolkit without fighting over it.

## Problems this closes

1. The lock does not record which dashcrew wrote it. Two teammates on different dashcrew versions rewrite the same files back and forth, and the guard rules can differ silently between them.
2. After a clone the guard binary is missing until the person runs `dashcrew sync`; until then no guard rule applies, because a hook command that fails is not a block. `doctor` reports it, but nothing tells the person why it matters.
3. There is no written guidance on what to commit, how each teammate onboards, or what to do after a merge that touched `.ai/project.yaml`.

## Not doing

- Making a missing guard block every tool call: a broken guard must never stop work, so the guard stays fail-open.
- Pinning dashcrew through `pubspec.yaml` `dev_dependencies`: that needs the pub.dev release (P4).
- Codex and Antigravity runtime verification (no model access); tracked in the P3 and P3c plans.

## Stages

### T1: version in the lock

- `lib/src/version.dart` holds `dashcrewVersion`; a test keeps it equal to `pubspec.yaml` `version`.
- `sync` writes `dashcrew_version` into `.ai/lock.json`. A lock written by a newer dashcrew stops `sync` with "update dashcrew, or rerun with --force"; an older or missing version is upgraded.
- `doctor` check `version`: ok when equal; warn when the lock is older ("run dashcrew sync"), newer ("update dashcrew to <x>"), or has no version.

### T2: team guide in the README

- What to commit (everything `sync` writes except `.ai/bin/`) and why: teammates get the toolkit on pull without running `sync`, and `doctor` can compare against the committed lock.
- Onboarding per agent: everyone runs `dashcrew sync` once (compiles the guard) and `dashcrew doctor`; Codex users trust the project and approve the hook in `/hooks`.
- After a merge that touched `.ai/project.yaml` or `.ai/packs/`: resolve the conflict in those sources only, then run `dashcrew sync` and commit the regenerated files.
- Everyone on the same dashcrew version; `doctor` says when not.
- What each agent enforces (role limits differ by agent).

### T3: missing guard after a clone

- `doctor` `guard-binary`, when the binary is missing but the lock and `.ai/guard.json` exist (a fresh clone), says that no guard rule applies until `dashcrew sync` compiles it.

## Order

T1 -> T2 -> T3. Each stage: tests first, `dart analyze --fatal-infos` and `dart test` on the minimum SDK and stable, goldens reviewed, commit by the maintainer.

## Progress

- T1 done. `dashcrewVersion` and `compareVersions` in `lib/src/version.dart` (a test keeps the constant equal to `pubspec.yaml`). The lock records `dashcrew_version`; `sync` stops with exit 1 on a lock written by a newer dashcrew unless `--force`, and upgrades an older or missing one. Doctor check `version` warns on any mismatch or a missing version, naming the fix. Every golden lock gains only the `dashcrew_version` line. Checked through the CLI on a scratch project.
- T2 done. README: status (pre-release; Claude Code run end to end, Codex and Antigravity checked against docs and Codex source only), install from git with `--git-ref` pinning (checked in a scratch pub cache: activate, then `sync` for all three agents compiles the guard and `doctor` passes besides the expected `codex-trust` warning), commands, and "Using dashcrew in a team": set up once, what to commit, each teammate, day to day, and what each agent enforces. The README says plainly that until numbered releases every commit reports version 0.0.1, so the version check cannot tell two commits apart.
