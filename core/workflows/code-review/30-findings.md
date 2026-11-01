---
target: workflow/code-review
order: 30
rules:
  - review.doc-comment-compliance
  - review.auto-reject
  - review.finding-format
  - review.severity-verdict
---
## Documentation

- Every unit the documentation rules require must carry a doc comment.
- Doc comments are written in `{{language.dartdoc}}` and are at most {{policy.dartdoc_max_lines}} lines.
- A missing doc comment, one over the line cap, and one that restates the logic are findings.

## Rejected automatically

These are process violations, reported without discussion:

- changed lines unrelated to the task (reformatting, whitespace, quote style)
- comments that explain what the code does
- a doc comment over {{policy.dartdoc_max_lines}} lines or not in `{{language.dartdoc}}`
- an abstraction with a single caller
- error handling for a case that cannot happen

## Finding format

Every finding has a severity, `file:line`, the current code and the fixed code.
A finding without an address is not a finding.

```
[BLOCKER] Order total ignores the discount
<data>/mappers/order_mapper:42

  total = response.subtotal

Fix:

  total = response.subtotal - (response.discount or 0)
```

## Verdict

```
REVIEW: <plan | n files>

BLOCKER  n
HIGH     n
MEDIUM   n
LOW      n

Criteria
  AC-1  met      <file:line>   <test file:line | manual item>
  AC-2  partial  <what is missing>

Verdict: [BLOCK / PASS WITH NOTES / PASS]
```

- LOW is a finding that changes neither behavior nor risk (naming, a small duplication).
- BLOCK when there is any BLOCKER or HIGH.
- PASS WITH NOTES when there are only MEDIUM or LOW findings.
- PASS otherwise.
