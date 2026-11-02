# P10b plan: personal profile (`me.yaml`)

Tier: large. Idea: `docs/ideas/knowledge-tracker-multirepo.md`, sections "Personal profile" and "Symlink layer". Runs after P10, so two stacks exist (Flutter and Go) and a role maps to different skills.

## Principle

The repo decides the rules; the person decides the experience. The guard never reads the profile, and a role grants no authority: a member editing a repo outside their role follows that repo's rules and guard exactly like its owners.

## Design (first version)

- `~/.dashcrew/me.yaml`, one per machine, never committed:
  ```yaml
  role: mobile        # be | fe | mobile | devops | fullstack | lead
  stacks: [flutter]   # optional, derived from role
  level: regular      # new | regular
  chat_language: id
  ```
- `dashcrew me init` proposes a role and stacks from the git log (the repos and paths the member commits to most); the member confirms or edits. `dashcrew me` prints the profile.
- Effects reach the agent through a git-ignored local file that `sync` writes next to the committed toolkit, never through AGENTS.md (shared by the team):
  - Claude Code: `CLAUDE.local.md`;
  - Codex and Antigravity: their local override, to be confirmed live; without one, the effect is documented as Claude Code only.
- Effects in the first version (all four confirmed):
  1. **Outside the role:** when the repo's platform is not one of the member's stacks, step 0 asks more and the reviewer always loads the repo's review checklist.
  2. **Onboarding (`level: new`):** before a task, the agent walks the relevant knowledge pages and, with `project.links`, the cross-repo flow.
  3. **Chat language:** `chat_language` overrides the profile's `language.chat` on this machine only.
  4. **Lead focus:** `role: lead` steers toward knowledge inbox review, a status summary, `audit` and PR review rather than writing code.
- **Skills per stack (confirmed for the first version):** role skills live in `~/.dashcrew/skills/<stack>/` and are linked into the agent's skills folder, git-ignored, so only the member's stacks are loaded. Requirements from the idea doc: the guard resolves real paths; link targets recorded in the lock; Windows falls back to a copy; each agent checked live for following linked skill folders; `.ai/` excluded from generated analyzer and indexing configs.

## Open before the plan is final

- Which skills are "role skills" (linked per person) and which stay team skills (committed): the split decides what a teammate without the stack still sees.
- Codex and Antigravity local override files.
- How `dashcrew me init` reads the git log across repos (the current repo only, or every repo under a folder).

## Progress

- [ ] Plan final (after P10)
