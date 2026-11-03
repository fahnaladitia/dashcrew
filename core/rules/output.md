---
target: agents_md
order: 90
rules: [debug.read-before-speculate, debug.one-pass, debug.unclear-say-so, format.no-em-dash, format.copy-paste-safe, format.chat-blocks, format.compare-as-table, workflow.no-narration]
---
## Debugging

- Read the code before judging a bug.
- In one pass: what you found, where, the fix.
- Cause unclear, or an error gone unexplained: say so; never guess or claim it fixed.

## Output format

- No em dashes, smart quotes or decorative Unicode: plain hyphens and straight quotes.
- Code output is copy-paste safe: real values, no `<x>` or `OWNER` placeholders (ask first); commands run together go on one line, joined with &&.
- Never echo raw tool output or narrate; show only changed code.
- Chat blocks (step 0, review, verify, summary) are `text` blocks: title, 72-column `=` rule, `Label  value` rows, each list under a `-- Name ---` rule, each item starting a row. Align with spaces, never `|` or boxes; commands sit alone on a line.
- A comparison (options, before and after, trade-offs) is a table in that layout, one row per item.
