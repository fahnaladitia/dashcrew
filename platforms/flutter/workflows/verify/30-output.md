---
target: workflow/verify
order: 30
rules:
  - verify.output-format
  - verify.record-tree
---
## Output

```
VERIFY: [PASS/FAIL]

Scope:    [N files analyzed, M test files]
Analyze:  [OK / N errors, M warnings]
Tests:    [X/Y passed (scoped)]
Manual:   [N items handed to the user / none]
Secrets:  [OK / N findings]
Arch:     [OK / N violations]
QA ids:   [OK / N widgets without identifier / N/A]
Diff:     [N files, M lines out of scope]

PR ready: [YES/NO]
```

- On NO, list every issue with `file:line` and a concrete fix.
- Paste the real command output under the summary. Never conclude PASS without it.
- On PASS, run `dashcrew verify record` as the last step: `dashcrew commit suggest` then lists the files changed after this verify.
