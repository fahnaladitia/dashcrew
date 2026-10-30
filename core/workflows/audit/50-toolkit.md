---
target: workflow/audit
order: 50
rules:
  - audit.toolkit-fixed-checks
  - audit.toolkit.t2-command-list
  - audit.toolkit.t3-agent-list
  - audit.toolkit.t4-readonly-tools
  - audit.toolkit.t6-skills-sourced
  - audit.skill-map
  - audit.toolkit-change-needs-yes
---
## Toolkit mode: `/audit toolkit`

Run the fixed checks T1 to T7, in this order, so the result repeats for the same commit.
Add no other dimension.

| Check | What |
|---|---|
| T1 | the agent's generated command, role and skill folders are complete, with no broken links |
| T2 | the workflow (command) list in AGENTS.md matches the generated workflow files |
| T3 | the role lists in AGENTS.md and in the team overview match the generated role files |
| T4 | read-only roles have no file-writing tools; roles that write plans may write only while the guard restricts them to `{{paths.plans}}`; a role with no tool restriction at all is reported |
| T5 | every skill that AGENTS.md, the workflows or the roles reference can be loaded in the current session |
| T6 | every installed skill has a recorded source (the skills lock file or the toolkit itself) and at least one reference; no deliberately excluded skill is installed |
| T7 | every hook script the agent settings reference exists on disk |

### Skill map

End with one table covering every loaded skill:

```
| Skill | Chain step | Role | Trigger | Verdict | Reason |
```

- Verdicts: `Keep`, `Improve`, `Update`, `Retire`, `Merge into <X>`.
- Each reason stands alone: for Retire, name the flaw and the replacement; for Merge, name the target and what moves.
- Usage frequency is not measurable here. Write "not measurable", never zero, and never use it as a reason to retire.

Change the toolkit (install, remove, or edit a skill, role or workflow) only after the user says yes.

### Output

```
AUDIT toolkit

T1 links           OK / <n> problems
T2 workflow list   OK / <n> problems
T3 role list       OK / <n> problems
T4 role tools      OK / <n> problems
T5 skills loadable OK / <n> not loaded
T6 skills sourced  OK / <n> problems
T7 hooks           OK / <n> problems

Skill map: <n> skills (Keep a, Improve b, Update c, Retire d, Merge e)

Top 3 actions:
1) ...

File: <audit file>
```
