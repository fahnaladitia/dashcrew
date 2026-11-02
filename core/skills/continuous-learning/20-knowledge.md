---
target: skill/continuous-learning
order: 20
when: { has_knowledge: true }
rules:
  - learn.knowledge-proposal-format
---
## Knowledge proposals

A product lesson goes to `.ai/knowledge/inbox/<yyyy-mm-dd>-<slug>.md`, one lesson per file:

```markdown
---
kind: feature | repo
target: <page slug: features/<slug>.md or repos/<slug>.md>
repo: <repo the lesson is about>
branch: <current branch>
commit: <current commit>
---

## <short heading>

<the fact and why it holds, with code pointers as `<repo>:<path>:<line>`>
```

Grep `.ai/knowledge/` for the page first and reuse its slug. Write nowhere else in `.ai/knowledge/`.
