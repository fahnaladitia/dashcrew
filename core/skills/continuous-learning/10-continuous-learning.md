---
target: skill/continuous-learning
order: 10
rules:
  - learn.one-fact-per-file
  - learn.index-one-line
  - learn.update-not-duplicate
  - learn.link-related
---
# Continuous learning

Goal: a correction given once is never needed again in a later session.
What to extract and what to skip is listed in the `learn` workflow.

## Where

Memory lives in `{{paths.memory}}/`, not in the source tree. It is session knowledge, not a project artifact.

## One fact per file

```markdown
---
name: <kebab-case-slug>
description: <one line, used to judge relevance at recall time>
type: user | feedback | project | reference
---

<the fact>

**Why:** <why it holds>
**How to apply:** <what to do differently>
```

- One file holds one fact.
- `Why` and `How to apply` are required for `feedback` and `project` memories. Without them the memory cannot drive an action.
- Link related memories with `[[slug]]`.

## Index

Add one pointer line per memory to `{{paths.memory}}/MEMORY.md`:

```
- [Title](slug.md) - short hook
```

- The index holds one line per memory.
- Never put memory content in the index.

## Before writing

- Search for an existing memory on the same topic.
- Update that file instead of creating a duplicate.
- Delete a memory that turned out to be wrong.

## Reminders are not the work

An end-of-session reminder, where the agent supports one, can only say the session was long enough to review.
It cannot read the transcript and extract patterns. The extraction is `learn`.
