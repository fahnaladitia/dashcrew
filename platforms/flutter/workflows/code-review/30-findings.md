---
target: workflow/code-review
order: 30
rules:
  - review.dartdoc-compliance
  - review.auto-reject
  - review.finding-format
  - review.severity-verdict
---
## Documentation

- Every unit the documentation rules require must carry Dartdoc.
- Dartdoc is written in `{{language.dartdoc}}` and is at most {{policy.dartdoc_max_lines}} lines.
- Missing Dartdoc, Dartdoc over the line cap, and Dartdoc that restates the logic are findings.

## Rejected automatically

These are process violations, reported without discussion:

- changed lines unrelated to the task (reformatting, whitespace, quote style)
- comments that explain what the code does
- Dartdoc over {{policy.dartdoc_max_lines}} lines or not in `{{language.dartdoc}}`
- an abstraction with a single caller
- error handling for a case that cannot happen

## Finding format

Every finding has a severity, `file:line`, the current code and the fixed code.
A finding without an address is not a finding.

```
[BLOCKER] BuildContext used across an async gap
lib/ui/orders/order_page.dart:88

  await repository.submit();
  Navigator.of(context).pop();

Fix:

  await repository.submit();
  if (!context.mounted) return;
  Navigator.of(context).pop();
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
