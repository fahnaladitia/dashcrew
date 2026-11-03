---
target: workflow/verify
order: 30
rules:
  - verify.output-format
  - verify.record-tree
---
## Output

```text
VERIFY  <PASS | FAIL>  PR ready: <yes | no>
========================================================================
Scope      <n> files analyzed, <m> test files
Analyze    <OK | n errors, m warnings>
Tests      <x>/<y> passed (scoped)
Manual     <n items handed to the user | none>
Secrets    <OK | n findings>
Arch       <OK | n violations>
QA ids     <OK | n widgets without identifier | N/A>
Diff       <n> files, <m> lines out of scope

-- Issues --------------------------------------------------------------
1     <file:line>
      <issue>. Fix: <concrete fix>
========================================================================
```

- With `PR ready: no`, `Issues` lists every issue with `file:line` and a concrete fix; otherwise it says `none`.
- Paste the real command output under the block. Never conclude PASS without it.
- On PASS, run `dashcrew verify record` as the last step: `dashcrew commit suggest` then lists the files changed after this verify.
