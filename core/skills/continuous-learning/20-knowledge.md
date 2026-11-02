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
target: <page slug, such as orders: the page is features/<slug>.md or repos/<slug>.md by kind>
repo: <repo the lesson is about>
branch: <current branch>
commit: <current commit>
---

## <the page section it belongs to>

<the fact and why it holds, with code pointers as `<repo>:<path>:<line>`>
```

Grep `.ai/knowledge/` for the page first and reuse its slug and a section it already has. A new page is made from `templates/page.md`: Purpose, Entry points, State and endpoints, Decisions, Gotchas; any other heading becomes a new section. Write nowhere else in `.ai/knowledge/`.
