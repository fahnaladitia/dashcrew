---
target: workflow/orchestrate
order: 90
rules:
  - orchestrate.final-output
  - roster.skip-shown-in-output
---
## Final output

```
ORCHESTRATE: <flow>
Roster: {{paths.plans}}/<feature>/handoff/00-roster.md

Phase 1 <role>  [DONE]   <n> files   skills: <used list>
Phase 2 <role>  [SKIP]   <evidence from the roster>
Phase 3 <role>  [DONE]   <n> files   skills: none
Phase 4 <role>  [ADDED]  <evidence from the phase that asked for it>  <n> files

flutter analyze: <result, scoped>
flutter test:    <result, scoped>

Open:
- <list>
```

- The number of phase lines equals the number of roles in the flow chain.
- SKIP phases stay in the output with their reason, so a reviewer can tell the question was asked.
