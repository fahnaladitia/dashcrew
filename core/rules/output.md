---
target: agents_md
order: 90
rules: [debug.read-before-speculate, debug.one-pass, debug.unclear-say-so, format.no-em-dash, format.copy-paste-safe, workflow.no-narration]
---
## Debugging

- Read the relevant code before saying anything about a bug.
- In one pass, state what you found, where, and the fix.
- If the cause is unclear, or an error disappeared without a known reason, say so. Never guess, never claim it fixed.

## Output format

- No em dashes, smart quotes or decorative Unicode: plain hyphens and straight quotes. Natural-language characters are fine.
- Code output must be copy-paste safe.
- Never echo raw tool output or narrate; show only the code that changed.
