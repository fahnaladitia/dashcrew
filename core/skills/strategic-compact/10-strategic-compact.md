---
target: skill/strategic-compact
order: 10
rules:
  - compact.phase-boundaries
  - compact.state-before
---
# Strategic compact

Automatic compaction fires at arbitrary points, often mid-task, and drops context that is still in use.
Compacting by hand at a phase boundary keeps what matters and drops what is finished.
A compaction suggestion from the agent tells you when; you decide whether.

## Good points

| Point | Drop | Keep |
|---|---|---|
| after exploration, before implementation | skimmed file contents, search output | the plan, the affected file list |
| after the data layer is green, before UI | test iteration detail, old test output | model shapes, repository signatures |
| after debugging is done | dead ends, stack traces, failed attempts | the root cause and its fix |
| before review | minute-by-minute implementation detail | changed file list, decisions taken |

## Bad points

- in the middle of a red-green-refactor cycle
- while a bug hunt has not found its root cause yet
- right before you need to re-read something just discussed

## Before compacting

Write this block to chat first:

```
STATE BEFORE COMPACT

Working on:     <task>
Done:           <list, with file paths>
In progress:    <what, in which file, at which line>
Decisions:      <binding decisions; do not decide them again>
Next:           <concrete next step>
Relevant files: <paths>
```

What you write explicitly is far more likely to survive than what is only implied across dozens of tool calls.

## With /orchestrate

Phase boundaries in `/orchestrate` are the best compact points there are.
The HANDOFF block is designed to be enough to continue after a compact. If it is not, the handoff is too thin.
