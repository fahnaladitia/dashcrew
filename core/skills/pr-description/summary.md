---
target: skill/pr-description
order: 400
rules: [summary.block, summary.pack-rows]
---
## Summary in chat

After the file is written and the PR question is answered, close the round in chat with this block. A trivial task keeps its one-line summary instead.

```text
SUMMARY
========================================================================
Skipped    <what and why | none>
Follow-up  <what | none>
Docs       <units that got a doc comment | none>
Tests      <x>/<y> passed (scoped), skipped: <n and why | none>
PR text    <description path>
PR         <URL | not created>

-- Changed -------------------------------------------------------------
FILE                    WHAT
<file>                  <what changed>

-- Commit --------------------------------------------------------------
<the commit command>
========================================================================
```

Rows the active packs add are listed below; each goes after `PR`, in the same layout.
