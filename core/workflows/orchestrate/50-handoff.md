---
target: workflow/orchestrate
order: 50
rules:
  - handoff.block-format
  - handoff.requires-paths
  - handoff.saved-to-file
  - handoff.path-not-content
  - handoff.no-silent-override
  - handoff.minimal-prompt
  - handoff.read-only-existing
---
## Handoff

Every phase closes with this block, and the next phase starts by reading it:

```
HANDOFF: <phase name>

Done:
- <what actually changed, with file paths>

Files touched:
- lib/... [new|changed]

Decided:
- <decisions that bind the next phase>

Open:
- <what was left undone on purpose, and why>

Skills used:
- <name> - <what for, one phrase>

Skills skipped:
- <name from the active list that turned out unnecessary> - <why>

For next phase:
- <concrete instructions, not a summary>
```

- A handoff that summarizes without file paths is invalid. The next phase needs addresses, not a story.
- Save each handoff to `{{paths.plans}}/<feature>/handoff/<role>.md`.
- Roles that may write save their own handoff. Read-only roles return it, and the main session saves it.
- A role starts with an empty context and reads files. The main session passes paths in the prompt, never copied content, and roles report paths, not file copies.
- A role that cannot follow an earlier decision writes it under "Open". The main session raises it to the user instead of choosing.

### Role prompt

The main session sends each role only this:

```
Feature: <folder name>
Base branch: <base>
Roster: {{paths.plans}}/<feature>/handoff/00-roster.md
Active skills: <the role's list from the roster, or "none">
Read: <existing handoffs, PRD folder, design spec>
Design link: <design-refiner only, from the user>
Task: <one sentence for this role>
Close with HANDOFF (including Skills used / Skills skipped) and save it to {{paths.plans}}/<feature>/handoff/<role>.md
```

- "Read" names only files that exist.
- A skipped role has no handoff. Receivers never wait for it and never invent it.
