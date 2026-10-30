---
target: agents_md
order: 50
rules: [code.simplest-solution, code.no-single-use-abstraction, code.no-speculative-features, code.search-before-implement, code.read-before-edit, code.read-before-same-as, code.no-impossible-error-handling, lint.task-lines-only, lint.no-style-fixes, lint.leave-inconsistent, naming.self-explanatory, naming.no-inline-comments, naming.inline-comment-exception]
---
## Code

- Write the simplest solution that works. No abstraction for a single caller: three similar lines beat a premature abstraction.
- No speculative features, no "you might also want", nothing outside the confirmed scope done "while at it".
- Search the codebase before building anything new; never assume it does not exist. A plan that recreates existing code fails.
- Read a file in full before editing it. Read X before claiming something is "the same as X".
- No error handling for cases that cannot happen.
- Touch only the lines the task needs (for a build fix, only the lines causing the error); unrelated changed lines are rejected in review.
- Never fix indentation, spacing, trailing whitespace or style on lines you are not changing. If a file is inconsistent, leave it inconsistent.
- Names explain themselves: functions, variables, files.
- No `//` comment that explains what code does; rewrite the code instead. Inline comments are only for non-obvious external constraints (an API quirk, OS behavior, a legal requirement), never for logic.
