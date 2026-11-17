# Launch direction: the team layer, told through the guard

Outcome of an idea-refine session on `docs/ROADMAP.md`, 2026-11-17. Chosen: the team layer as the substance, the guard as the story, launch before the 1.0 freeze.

## Problem Statement

How might we make dashcrew the tool mixed-agent teams trust to run their AI coding agents, so that a visitor from GitHub Trending understands it in 30 seconds, tries it in one command, and keeps using it?

## Recommended Direction

**Build for teams that mix agents, and lead with the guard.**

The durable difference from rulesync and Ruler is the team layer they lack:

- a lock and drift check in CI (`dashcrew check`);
- team and personal profiles;
- commit lint;
- one guard policy for every agent.

That is what keeps a team on dashcrew, but it does not fit in a 10-second GIF. The guard does: an agent runs `bash -c "git push"`, and dashcrew blocks it in Claude Code, Codex and Cursor alike. So the README opens with the guard, and the team layer is the second screen.

**Launch once Cursor is supported, and before the 1.0 freeze.**

- A "for any multi-agent team" pitch without the most used agent loses most visitors on the first scroll.
- CLI renames, schema 2 and internal refactors are invisible to visitors. They are better decided with feedback from real users.

**Frame the guard honestly.** From its first line, the guard is presented as protection against accidents, not a sandbox, with its known misses listed in plain sight. The first "bypass in 5 seconds" comment then finds that we already say so.

## Key Assumptions to Validate

- [ ] **Native deny rules miss nested shells, and dashcrew does not.** Test: one hour in Claude Code and Codex, with `Bash(git push:*)`-style deny rules against `bash -c "git push"`, `sh -c` and `env git push`. If the native rules catch them, the pitch becomes "one policy for every agent" instead.
- [ ] **Cursor's hooks can run the guard fail-closed.** Test: a spike adapter, then the doctor guard probe on a real Cursor install.
- [ ] **We know which shell each agent runs hooks with on Windows.** Test: 1-2 days on a Windows machine with Claude Code, Codex and Cursor. The answer decides whether the `sh` shim survives or a `.cmd` shim is needed. Today a hook that calls `dashcrew` directly fails open when the binary is missing: exit 127 is not the exit 2 that blocks.
- [ ] **The office team's "unstable" means forced re-syncs and a silent guard, not code bugs.** Test: 15 minutes with each teammate, asking for their last three frustrations.
- [ ] **Trending visitors become users.** Test: for 2 weeks after launch, track release downloads, issues and Discussions from new people, not just stars.

## MVP Scope (launch-ready, Tuesday 2027-01-12)

In:

- **v0.7 Trust** (until ~2026-11-28):
  - crash-safe sync;
  - the doctor guard probe;
  - a visible warning when the guard fails open;
  - the Windows hook investigation;
  - a release checklist and an `-rc` tag before each minor.
- **v0.8 Launch-ready** (until ~2026-12-19):
  - a README under 150 lines that opens with the guard GIF;
  - `dashcrew guard init`: guard hooks and a default policy without a `project.yaml`, as the 10-second way to try it;
  - a Cursor adapter (rules plus the guard hook);
  - `install.sh`;
  - `init` without dead ends (Go detected, defaults for `other`, a clean doctor on a fresh project);
  - repo description, topics, social preview and Discussions;
  - `docs/adding-an-adapter.md` and 5+ good first issues.
- **Launch:**
  - a soft launch in December to the office team and a few outside developers;
  - Show HN, Reddit and X on 2027-01-12, with someone answering issues for 24 hours.

Out: everything below.

## Not Doing (and Why)

- **The 1.0 freeze before launch.** CLI renames, schema 2 and refactors are invisible to visitors, and real users should shape what gets frozen. 1.0 follows about 4-6 weeks after launch.
- **Racing to 15+ adapters.** We cannot win on count. Cursor comes now, Gemini CLI as the first outside contribution against `adding-an-adapter.md`, and Copilot after that.
- **Import from rulesync and Ruler before launch.** Their users are a minority of our visitors. Build it if they ask.
- **A safety pack (`rm -rf`, `.env`, `curl | sh`) before launch.** Honest framing costs a day. A pack costs weeks and widens the attack surface the launch invites.
- **A pack registry.** It needs a community first.
- **An inter-process sync lock and a lock journal.** Nothing runs sync concurrently, and the crash-safe sync plan recovers without them.
- **P8 bridge, P13d and the multi-repo ideas.** They wait for users to ask.

## Open Questions

- Does `guard init` write project hooks, which fit the team story but need a commit? Or user-level hooks (`~/.claude` and the like), which fit a quick try and guard every repo?
- What version number ships at launch: v0.9.0, or v1.0.0-rc.1 to signal maturity?
- Who covers issues and comments on launch day, alongside the maintainer?
- Does Antigravity have a Windows build that needs the same hook check?
