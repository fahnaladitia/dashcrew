---
target: agents_md
order: 10
rules: [workflow.run-chain-by-default, workflow.chain-applicability, workflow.chain-no-skip, workflow.step0-no-blind-execution, workflow.step0-display-before-plan, workflow.step0-block, workflow.step0-user-confirms-once, workflow.step1-branch-setup, workflow.step1-branch-name-fixed, workflow.chain-order, workflow.trivial-edit-shortcut, workflow.subagents-two, workflow.loop-stops-on-criteria, workflow.review-severity-handling, workflow.rework-one-round, workflow.manual-checklist, pr.done-requires-file, pr.every-round, pr.create-ask, pr.ends-in-question, workflow.summarize-changes, workflow.summarize-dartdoc-units, workflow.summarize-pr, workflow.summarize-tests, workflow.learn-end-of-session, workflow.command-set-closed, workflow.build-fix-is-a-branch, workflow.audit-outside-chain, workflow.worktree-outside-chain]
---
## Workflow chain

Run this chain by default; the user never has to name a step. Never skip a step of your tier's chain, and finish each step before the next starts.

### Step 0

Never run a raw prompt as given: polish it with `prompt-polish` and print this block before anything else runs:

```text
TASK:             <feat | fix | hotfix>: <polished request>
Tier:             trivial | small | standard | large   (<evidence>)
Assumptions:      <each one, or none>
Questions:        <all of them, once, or none>
Skills:           <name> (<why>), <name> (<why>)
Success criteria: <id> <criterion> -> unit | ui | integration | manual
Branch:           1) <name>  2) <name>  3) <name>   base: <ask>
```

The user confirms or edits it once. Then create the branch (see Branching); its name is final, since the PR description path depends on it.

### Chain after step 0

| Tier | Chain |
|---|---|
| trivial | edit -> `{{platform.commands.analyze}}` on the file -> one-line summary |
| small | `tdd` (test first, then implement) -> scoped analyze and test -> summary |
| standard | `plan` (one file) -> `tdd` -> `code-review` (`reviewer` on the diff) -> fix -> `verify` -> PR description |
| large | `plan` -> user confirms -> `reviewer` on the plan -> `tdd` -> `code-review` -> at most one rework round -> `verify` -> PR description |

- Two read-only subagents exist: `reviewer` (reviews a plan or a diff) and `explorer` (maps many files to `file:line`). The main session does everything else.
- Loops stop on the step 0 success criteria, not when a reviewer runs out of findings. BLOCKER and HIGH are fixed; MEDIUM and LOW go to the PR Notes, asked about once as one list.
- A re-review covers only the rework diff. A second failing review goes to the user as a decision, never into a third round.
- Criteria mapped to `manual` form one manual checklist, handed to the user at `verify` and recorded in the PR Testing section.
- Work on a branch is finished only when its PR description is written or updated (`pr-description`), on every round, small tasks included. Then ask once whether to open a PR (`pr-create`); never open one without an explicit yes, and a yes never carries over to another branch or round.
- The summary gives what changed, was skipped and needs follow-up; every unit that got a doc comment; test status, skipped tests included; the PR description path and the PR decision (the URL, or "not created"). End the session with `learn`.

- Only these workflows exist: `plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `worktree`, `build-fix`. If a request names any other, say it does not exist; never improvise one.
- `build-fix` is a detour, not a step: take it when a build, dependency or code generation error blocks a step, then resume where you stopped.
- `audit` (read-only) and `worktree` sit outside the chain. Each audit finding becomes a new task; each worktree session runs the chain itself, and an inline task is run by the main session.
