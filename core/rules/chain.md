---
target: agents_md
order: 10
rules: [workflow.chain-no-skip, workflow.chain-applicability, workflow.run-chain-by-default, workflow.chain-order, workflow.step0-no-blind-execution, workflow.step0-display-before-plan, workflow.step1-checkpoint-baseline, workflow.step1-branch-setup, workflow.step1-branch-name-fixed, pr.done-requires-file, pr.every-round, pr.create-ask, pr.ends-in-question, workflow.summarize-changes, workflow.summarize-dartdoc-units, workflow.summarize-pr, workflow.summarize-tests, workflow.trivial-edit-shortcut, workflow.same-chain-for-small-work, workflow.command-set-closed, workflow.build-fix-is-a-branch, workflow.audit-outside-chain, workflow.worktree-outside-chain, workflow.orchestrate-replaces-plan-to-review, workflow.team-changes-actors-only, pr.orchestrate-not-replacement, workflow.when-to-use-team]
---
## Workflow chain

Run this chain by default for any work that touches logic, state, or more than one file; the user never has to name a step. Never skip a step, and never start a step before the previous one is complete.

0. **Prompt polish** - never execute a raw prompt as given. Polish it with the `prompt-polish` skill and print the block in chat before `plan` runs.
1. **`checkpoint`** - create the baseline `<task>-start`, scoped to the folders the task will touch. Set up the branch (see Branching). The branch name chosen here is final: the PR description path depends on it.
2. **`plan`** - acceptance criteria, boundaries, affected files, leak analysis, Dartdoc targets. Ends with `Verdict: GO | NO-GO`.
3. **`tdd`** - data-layer tests first, then the rest.
4. **`code-review`** - covers the uncommitted working tree and `<base>...HEAD`.
5. **`verify`** - scoped analyze and tests, with evidence.
6. **`checkpoint` verify** - compare against the baseline: better, same, or worse.
7. **PR description** - use the `pr-description` skill. Run it on every finished round of work on the branch; work is not finished until that file is created or updated.
8. **PR create** - ask once whether to open a PR. Never open one without an explicit yes, and a yes never carries over to another branch or round. Use the `pr-create` skill.
9. **`learn` and summary** - the summary states what changed, what was skipped, and what needs follow-up; names every unit that got Dartdoc; reports test status, including skipped tests; and gives the PR description path plus the PR decision (the URL, or "not created").

- A typo or a single-line copy change skips the chain: make it, then summarize in one line.
- Small work (one component, one card, mock data) runs every step. Only the scope of each step shrinks, never the number of steps.
- Only these workflows exist: `checkpoint`, `plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `orchestrate`, `worktree`, `build-fix`. If a request names any other, say it does not exist; never improvise one.
- `build-fix` is a detour, not a step: take it when a build, Gradle, CocoaPods or build_runner error blocks a step, then resume where you stopped.
- `audit` sits outside the chain and is read-only; each of its findings becomes a new task that runs the chain.
- `worktree` sits outside the chain: it only prepares one worktree per parallel task. Each worktree session runs the chain itself; an inline task is run by the main session.
- `orchestrate` replaces steps 2-4 for large work; never run both. Its team flow changes who performs steps 2-4 and 7, never the chain. Checkpoint, verify, the PR question and the PR description (written after the flow, from the real diff) stay with the main session.
- Use the team flow for work that arrives with a PRD, a tracker task or a design link, or that spans several layers. Use the feature, bugfix or refactor flow for small fixes. The main session makes a one-file or copy change directly.
