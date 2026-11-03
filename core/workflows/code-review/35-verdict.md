---
target: workflow/code-review
order: 35
rules: [review.severity-verdict]
---
## Verdict

The review prints as one block, the verdict first:

```text
REVIEW  <BLOCK | PASS WITH NOTES | PASS>  (<plan | n files>)
========================================================================
BLOCKER <n>  HIGH <n>  MEDIUM <n>  LOW <n>

-- Findings ------------------------------------------------------------
<each finding in the finding format, most severe first | none>

-- Criteria ------------------------------------------------------------
ID    STATUS   WHERE                PROOF
AC-1  met      <file:line>          <test file:line | manual item>
AC-2  partial  <what is missing>
========================================================================
```

- LOW is a finding that changes neither behavior nor risk (naming, a small duplication).
- BLOCK when there is any BLOCKER or HIGH.
- PASS WITH NOTES when there are only MEDIUM or LOW findings.
- PASS otherwise.
