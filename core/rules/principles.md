---
target: agents_md
order: 49
rules: [principle.think-before-coding, principle.simplicity, principle.surgical-changes, principle.goal-driven, principle.always-ask, code.simplest-solution, code.no-speculative-features, lint.task-lines-only]
---
## Principles

- **Think before coding.** State assumptions; when the request reads two ways, name both, never pick one silently. Say so when a simpler approach exists. Ask every question at step 0, together; one found later, when found. The main session always asks open questions, with the agent's question tool when it has one, and waits.
- **Simplicity.** The minimum that solves the request, in code and documents. A plan or doc that could be a third of its length is rewritten.
- **Surgical changes.** Every changed line traces to the request. Remove imports, variables and functions your change made unused. Mention unrelated dead code; never delete it unasked.
- **Goal-driven.** Turn the task into verifiable criteria first. Loop until they pass, then stop.
