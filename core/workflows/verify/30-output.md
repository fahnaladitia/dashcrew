---
target: workflow/verify
order: 30
rules:
  - verify.output-format
---
## Output

```
VERIFY: [PASS/FAIL]

Scope:    [N files analyzed, M test files]
Analyze:  [OK / N errors, M warnings]
Tests:    [X/Y passed (scoped)]
Secrets:  [OK / N findings]
Arch:     [OK / N violations]
QA ids:   [OK / N widgets without identifier / N/A]
Diff:     [N files, M lines out of scope]

PR ready: [YES/NO]
```

- On NO, list every issue with `file:line` and a concrete fix.
- Paste the real command output under the summary. Never conclude PASS without it.
