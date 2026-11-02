---
target: agents_md
order: 10
rules: [workflow.run-chain-by-default, workflow.chain-applicability, workflow.chain-no-skip, workflow.step0-no-blind-execution, workflow.step0-display-before-plan, workflow.step0-block, workflow.step0-user-confirms-once, workflow.step1-branch-setup, workflow.step1-branch-name-fixed, workflow.chain-order, workflow.trivial-edit-shortcut, workflow.subagents-two, workflow.loop-stops-on-criteria, workflow.review-severity-handling, workflow.rework-one-round, workflow.manual-checklist, pr.done-requires-file, pr.every-round, pr.create-ask, pr.ends-in-question, workflow.summarize-changes, workflow.summarize-dartdoc-units, workflow.summarize-pr, workflow.commit-command-step, workflow.summarize-tests, workflow.learn-end-of-session, workflow.command-set-closed, workflow.build-fix-is-a-branch, workflow.audit-outside-chain, workflow.worktree-outside-chain, workflow.step0-release, workflow.port-outside-chain, workflow.spec-with-knowledge, workflow.step0-spec]
---
## Workflow chain

Run this chain unasked. Never skip a step of your tier's chain; finish each before the next.

### Step 0

Never run a raw prompt: polish it with `prompt-polish` and print this block:

```text
TASK:             <feat | fix | hotfix>: <polished request>
Tier:             trivial | small | standard | large   (<evidence>)
Assumptions:      <each one, or none>
Questions:        <all of them, once, or none>
Skills:           <name> (<why>), <name> (<why>)
Success criteria: <id> <criterion> -> unit | ui | integration | manual
Branch:           1) <name>  2) <name>  3) <name>   base: <ask>
{{#has_release_lines}}
Release:          <version> | none
{{/has_release_lines}}
{{#has_knowledge}}
Spec:             specs/<release>/<feature>@rev<n> | none
{{/has_knowledge}}
```

The user confirms or edits it once. Then create the branch (see Branching); its name is final.

### Chain after step 0

| Tier | Chain |
|---|---|
| trivial | edit -> `{{platform.commands.analyze}}` on the file -> one-line summary |
| small | `tdd` (test first, then implement) -> scoped analyze and test -> summary |
| standard | `plan` (one file) -> `tdd` -> `code-review` (`reviewer` on the diff) -> fix -> `verify` -> PR description -> commit command |
| large | `plan` -> user confirms -> `reviewer` on the plan -> `tdd` -> `code-review` -> at most one rework round -> `verify` -> PR description -> commit command |

- Two read-only subagents: `reviewer` (a plan or a diff) and `explorer` (maps many files to `file:line`); the main session does the rest.
- Loops stop when the step 0 success criteria pass, not when findings run out. BLOCKER and HIGH are fixed; MEDIUM and LOW go to the PR Notes, asked about once as one list.
- A re-review covers only the rework diff. A second failing review goes to the user, never a third round.
- Criteria mapped to `manual` form one manual checklist, handed over at `verify`, recorded in the PR Testing section.
- A branch's work is done only when its PR description is written or updated (`pr-description`), every round, small tasks included. Then ask once whether to open a PR (`pr-create`); never without an explicit yes, which never carries over to another branch or round.
- The summary gives what changed, was skipped and needs follow-up; every unit that got a doc comment; test status, skipped tests included; the PR description path, the commit command and the PR decision (the URL, or "not created"). End the session with `learn`.

- Only these workflows exist: `plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `worktree`, `build-fix`{{#has_release_lines}}, `port`{{/has_release_lines}}{{#has_knowledge}}, `spec`{{/has_knowledge}}. Say any other named one does not exist.
- `build-fix` is a detour, not a step: take it when a build, dependency or code generation error blocks a step, then resume.
- `audit` (read-only){{#has_release_lines}}, `port`{{/has_release_lines}} and `worktree` sit outside the chain. Each audit finding becomes a new task; each worktree session (the main session for an inline one) runs the chain.
