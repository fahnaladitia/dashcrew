# P7 plan: specs (PRD and TRD)

Tier: large (a new knowledge folder with its own format, a guard rule change, new commands, a new workflow, step 0 and reviewer changes). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P7. Builds on P5 (knowledge repo, guard, MCP) and P6 (releases).

## Goal

A team writes the PRD and TRD of a large feature up front, reviews them once, and stays in step with them while several members build it in several repos. Writing the documents is not the hard part. The hard part is noticing when the work drifts from them. So the plan has two parts: one source of truth, and drift that is detected and declared.

## What exists

- **P5 knowledge repo:** `features/`, `repos/`, `inbox/`, a page parser, `knowledge review` and `knowledge doctor`, and the MCP `knowledge serve`. The guard allows agent writes only under `inbox/`.
- **P6:** releases per task, recorded in `branch.<name>.dashcrew-release`; the step 0 `Release:` line, gated on `has_release_lines`.
- **Skills:** `acceptance-criteria` (Given/When/Then, error states) and `feature-architecture` (widget split, state, DI, QA identifiers), with platform fragments.
- **Step 0** already holds `Success criteria: <id> <criterion> -> unit | ui | integration | manual`.
- **`reviewer`** reviews plans (tier large) and diffs, and maps criteria to `file:line`.

## Design

```
<knowledge repo>/specs/<release>/<feature>/
  prd.md   problem, users, goals, scope, Not Doing, AC-1..AC-n
  trd.md   impact per repo and platform, contracts C-1.., data and migrations,
           rollout order, risks, decisions D-1.., open questions with owners
```

- `<release>` is a version, or `unreleased` when the team has no release lines.
- Front matter on both files:
  - `title` and `feature`;
  - `release`;
  - `status`: `draft`, `review`, `approved` or `superseded`;
  - `rev`: an integer, starting at 1;
  - `repos`;
  - `owners`, as role names only (never names or emails).
- IDs are written at the start of a list item or heading: `AC-3`, `D-2`, `C-1`. They are never renumbered. A dropped one stays, struck through with a reason (`~~AC-3~~ dropped: <why>`).

## Stages

### S0: format and lint

- `internal/spec`: parse the front matter, sections and IDs; tests over fixtures.
- `dashcrew spec lint [<path>] [--against <git ref>]` reports:
  - missing front matter keys;
  - an unknown status;
  - duplicate IDs;
  - IDs that are not sequential when the spec is new;
  - a status of `approved` with open questions that have no owner;
  - with `--against`: an ID that disappeared, or one whose number changed meaning (same text moved to another ID), and a `rev` that did not increase while an approved file changed.

  Exit 1 on any error.
- `knowledge doctor` runs spec lint over `specs/`.

### S1: scaffold and guard

- `dashcrew spec new <feature> [--release <v>]` writes `specs/<release>/<feature>/prd.md` and `trd.md` from templates in the knowledge clone, with `status: draft` and `rev: 1`. It refuses if the folder exists. The release defaults to the branch's recorded one.
- Guard: agents may write under `specs/` in the knowledge clone only to a file that does not exist yet, or one whose current front matter says `draft` or `review`. Writes to `approved` or `superseded` files stay blocked, with a reason pointing to `dashcrew spec revise`. Every other path in the clone keeps the P5 rule (only `inbox/`).
  - The guard reads at most the first 2 KB of the target file to find the status, so latency stays within the current numbers.
- `dashcrew spec revise <path>` (run by the member) sets an approved spec back to `review` and increases `rev` in both files. It prints the git commands for the member to commit; it never runs them.
- `knowledge init` adds `specs/` and the two templates to new repos. Existing clones get them from `spec new` on first use.

### S2: the `spec` workflow

A new catalog workflow, gated on `has_knowledge`, run by the main session:

1. **Brainstorm:** problem, users, options with trade-offs, assumptions, Not Doing. Every question is asked in one round.
2. **PRD:** with `acceptance-criteria`; every AC gets an ID.
3. **TRD:** with `feature-architecture` and the platform's TRD fragment (a new `trd` fragment target per platform; Flutter: state, routing, identifiers; a level 1 platform gets the generic sections only). Contracts C-n and decisions D-n get IDs.
4. Run `dashcrew spec lint`.
5. Print the git commands for the member to open a PR on the knowledge repo (status `review`). Approval is the merged PR, with `status: approved` set in that PR by the member.

The workflow is added to the closed workflow list.

### S3: tasks follow the spec

- Step 0 gains `Spec: specs/<release>/<feature>@rev<n> | none`, gated on `has_knowledge`. The task's success criteria are taken from the spec's ACs by ID (`AC-2 ... -> unit`).
- The branch records it: `git config branch.<name>.dashcrew-spec specs/<release>/<feature>@rev<n>`.
- The `plan` workflow:
  - names the spec and revision;
  - lists the ACs this task covers;
  - states every departure from the TRD as `departs from D-n: <what and why>`.
- `reviewer`:
  - checks that every AC the task claims is covered with `file:line` and a test;
  - checks that a change against a TRD decision or contract is declared in the plan. An undeclared departure is HIGH.
- A declared, accepted departure is followed by a `spec revise` PR. The summary step says so.
- AGENTS.md: the `Spec:` line is paid for with cuts in the same file. The heaviest combination stays at or below 23,788 bytes.

### S4: drift between spec and work

- `dashcrew spec status [-p]`:
  - the branch's recorded spec and revision against the clone's current revision and status;
  - the ACs the plan claims;
  - the ACs no branch claims yet. It finds them from the plans folder of the current repo; other repos are listed as "not checked".
- `doctor` warns when the recorded revision is older than the current one ("spec moved to rev 4; re-read it"), or when the spec is `draft` while the task is past planning.
- `knowledge serve` already indexes every `.md` file; add a `kind` (`spec`, `feature`, `repo`) to search results, and let `knowledge_search` filter by kind.

### S5: retro and README

- The knowledge-gated `learn` text: after a feature ships, propose its `features/<slug>.md` page from the spec and the code through `inbox/`. Use the PRD's goals and the TRD's final decisions, and point to the code. Mark the spec `superseded` only when a newer spec replaces it.
- README section "Specs".

## How we check

- Unit tests:
  - the parser and lint over fixtures (good, duplicate ID, renumbered against a git ref, approved with an unowned question, rev not increased);
  - the guard (new file allowed, draft allowed, review allowed, approved blocked, superseded blocked, inbox unchanged, other clone paths unchanged), with latency re-measured;
  - `spec new` and `spec revise` in a temp clone;
  - `spec status` with an older recorded revision.
- Transcripts for `spec new`, `spec lint`, `spec revise`, `spec status` and a blocked agent write.
- Goldens: the step 0 `Spec:` line and the `spec` workflow in `sample-knowledge`; other samples unchanged.
- Live: in a test knowledge repo, run the `spec` workflow for a small feature, merge it as approved, then:
  1. start a task that names the spec;
  2. let the plan declare one departure;
  3. confirm the reviewer flags an undeclared one;
  4. run `spec revise`, and see `doctor` warn on the older branch.

## Not doing

- An approval command or bot; approval is a merged PR.
- Specs outside the knowledge repo (in app repos).
- Syncing specs to a tracker or document tool (a P8 bridge may do it later).
- Generating code or tests from the spec.
- Names or emails of people in specs; owners are roles.

## Decisions (confirmed)

1. Agents may write under `specs/` only to new files and to files in `draft` or `review`. `approved` and `superseded` files are blocked, and change through `dashcrew spec revise` run by a member.
2. Approval is the merged knowledge-repo PR in which the member set `status: approved`. There is no approve command.
3. A task links to its spec through the step 0 `Spec:` line, recorded in `branch.<name>.dashcrew-spec`.
4. A branch on an older revision gets a `doctor` warning; CI is not blocked.

## Progress

- S0-S2 done.
  - `internal/spec`:
    - front matter checked (keys, status, `rev >= 1`);
    - IDs only at the start of a list item or heading, outside code fences; `~~ID~~ dropped: <why>` counts as kept;
    - open questions under an "Open questions" heading, each with `owner: <role>`;
    - an email in owners is a problem.
  - `dashcrew spec lint [--against]` exits 1 on a problem or a bad ref, 64 on usage. `knowledge doctor` lints `specs/`.
  - `spec new` takes the release from the flag, then the recorded release, then `unreleased`. `spec revise` moves both files to review, sets the rev to one past the highest, and prints `git -C` commands. `knowledge init` adds `specs/` and the PRD and TRD templates.
  - Guard:
    - under `specs/`, only a missing path or a file in draft or review (read from its first 2 KB);
    - an agent running `dashcrew spec revise` is blocked when knowledge is set;
    - fixed an old gap: relative paths are no longer cleaned before links are resolved (`docs/alias/../repos/x.md`).
    - Latency medians about 6.4 ms.
  - Workflow `spec`, gated on `has_knowledge`. A new `trd` fragment target (`core/trd/`; Flutter adds State, Routing and QA identifiers).
  - AGENTS.md: 23,783 bytes at most (both features set).
  - Transcripts `spec-new`, `spec-lint` and `spec-revise`. Knowledge-set samples and transcripts change as expected. 21 new tests.
  - Open: `apply_patch` checks a moved file only at its new path, so a protected page or spec could be moved into `inbox/`. Fixed in S3-S5.
- S3-S5 done, plus the guard fix.
  - Guard: a patch move is also checked at its source, as a contentless write under the same rules. An approved spec or a feature page moved into `inbox/` is blocked, CRLF included.
  - S3:
    - the step 0 `Spec:` line (knowledge only);
    - `prompt-polish` takes the success criteria from the ACs;
    - `branching` records `branch.<name>.dashcrew-spec`;
    - `plan/30-spec.md`: an undeclared departure makes the verdict NO-GO;
    - `reviewer`: claimed ACs need `file:line` and a test, and an undeclared TRD departure is HIGH.
    - Helpers `RecordedSpec`, `RecordSpec` and `OwnCommits` in `internal/lines`.
  - S4:
    - `dashcrew spec status` reports the recorded revision against the clone, claimed and unclaimed ACs, and other repos as "not checked".
    - doctor `spec` check (warning only).
    - `knowledge_search` results gain `kind`, with an optional filter. Spec files are now indexed too.
  - S5: the `learn` text proposes `features/<slug>.md` after release. README section "Specs".
  - AGENTS.md: 23,784 bytes at most with both features. With knowledge, the memory line is "Read `.ai/memory` (never rewrite) and `.ai/knowledge`; check pointers." and `spec-driven-development` leaves the outside-skills list. The plan workflow, the explorer and the MCP tools still spell out verifying pointers.
  - Transcript `spec-status`. 13 new tests. Full suite passing.

P7 is complete in code. Left: the live check in "How we check".
