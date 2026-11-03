---
target: workflow/learn
order: 20
when: { has_knowledge: true }
rules:
  - learn.knowledge-proposal
  - learn.rules-not-knowledge
  - learn.knowledge-main-line
  - learn.feature-page-from-spec
  - learn.spec-superseded-only-when-replaced
---
## Product knowledge

The team's reviewed product knowledge is in `.ai/knowledge/`. Route each candidate:

- A product lesson (a flow, a contract, a trap, a decision and its reason) becomes one proposal, `.ai/knowledge/inbox/<yyyy-mm-dd>-<slug>.md`, in the format of the `continuous-learning` skill. Never edit the pages; a human reviews the inbox.
- A standing instruction for agents is a rule, not knowledge: propose it for the project pack in `.ai/packs/`.
- After a feature with a spec ships, propose its `features/<slug>.md` page the same way, through the inbox: the PRD's goals and the TRD's final decisions (`D-n`, with accepted departures), each with code pointers. The spec stays as written; it is marked `superseded` only when a newer spec replaces it.
{{#has_release_lines}}
- Pages describe the main line. Tag a version-specific part in place, such as `(since 2.5.0)` or `(2.4 only)`; never a folder or page per version.
{{/has_release_lines}}

In the `LEARN` block, list the proposals in a `-- Proposed ---` section after `Candidates`, before the closing `=` rule, one path per row.
