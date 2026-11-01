---
target: workflow/tdd
order: 15
rules:
  - tdd.main-session-implements
  - tdd.read-order
  - tdd.plan-scope-only
  - tdd.ui-from-spec
  - tdd.no-silent-override
  - tdd.analytics-needs-decision
---
## Implementing

The main session implements. Work the plan lists gets done; anything else does not.

Before writing a line, read in order: the plan (standard and large), its Architecture and Design sections when present, and every file you will change, in full.

- UI from the design spec, not from memory: the listed tokens and components, verbatim copy, no hardcoded server-driven text, identifiers from the plan, bounded image caches in lists, and every resource released.
- Doc comments in the same edit as the code, per AGENTS.md.
- A decision in the plan you cannot follow is raised to the user, never changed silently; the PR notes record it.
- Wire analytics only after the user's decision at step 0.
