---
target: workflow/audit
order: 40
rules:
  - audit.investigation
---
## Investigation mode: `audit [<task id or url>] <question>`

Example: `audit why does the order list keep the old status after going back from the order detail page`.

One question, one answer: the root cause with evidence, not a general audit.
Read the code first, then state the finding, its location and the fix. If the cause is unclear, say so; never guess.

1. With a task: read the task, comments and screenshots as in flow mode Phase 0. The user's question wins over the task text when their focus differs. Without a question, restate the task's main complaint as one sentence.
2. Staleness: compare the report date with `git log -S "<symbol>"` on the base. If a fix merged after the report, stop with verdict `ALREADY FIXED`.
3. Derive the flow boundaries from the question, without asking for flags, and write them in chat as one line. Example: "after going back from the order detail page" gives `--from` = return from the detail page, `--to` = the status shown in the order list.
4. Trace the reproduction path step by step: tap -> event -> state -> repository -> payload -> response -> mapper -> UI. Give `file:line` at every step and mark where the expected value turns into the wrong one.
5. Run only the F checks that touch the hypothesis, usually F2 state, F3 navigation (the result of a pop, a `then` that does not wait) and F5 mapper. Do not run or write the rest.
6. Use no subagents, unless the trace crosses more than 10 files. Then an `explorer` may map the path, and one `reviewer` checks the step 4 trace.

Use the flow mode verdicts (`VALID`, `ALREADY FIXED`, `BACKEND`, `AS DESIGNED`, `NEEDS INFO`).
Write the fix as a proposed diff; never apply it. The audit changes no tracker status.

### Output

```
AUDIT <task id or question slug>: <question in one sentence>
Boundary: <from> -> <to>

Trace:
1. <step> - lib/...:<line>
2. <step> - lib/...:<line>   <- the wrong value starts here

Root cause: <one or two sentences>
Verdict:    VALID / ALREADY FIXED <sha> / BACKEND / AS DESIGNED / NEEDS INFO <what>
Proposed fix: <short diff, not applied>
Test:       <the data-layer test that would catch this bug>

Draft tracker comment (not sent, under 1000 characters, only with a task):
<problem, evidence, request>

File: <audit file>
```
