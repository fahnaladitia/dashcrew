---
target: workflow/build-fix
order: 15
rules: [build.one-error-one-fix, build.output]
---
## Rules

- One error, one fix, one rebuild. Never stack fixes before rebuilding.
- If an error disappears and you do not know why, say so. Do not claim it is fixed.
- Touch no line outside the cause of the error.

## Output

```text
BUILD FIX
========================================================================
Error      <original message, trimmed>
Cause      <root cause, at file:line>
Fix        <what changed>

-- Verify --------------------------------------------------------------
<command run>
========================================================================
```

Paste the command's real output under the block. A claim without output is not verification.
