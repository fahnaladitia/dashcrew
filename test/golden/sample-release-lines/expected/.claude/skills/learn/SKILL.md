---
name: learn
description: "Extract reusable patterns from this session and save them as memory, or say that nothing is worth saving."
argument-hint: "[topic to focus on]"
---

# learn

Run `learn` at the end of every session. It can also run mid-session after a hard-won fix.
The file format and index rules live in the `continuous-learning` skill; apply it.

## Extract

| Kind | Example |
|---|---|
| `error_resolution` | a recurring code generation error and the fix that actually works |
| `user_corrections` | a correction the user had to give more than once |
| `workarounds` | odd behavior of a vendored or pinned package and how to work around it |
| `project_specific` | a convention written neither in AGENTS.md nor in the code |
| `api_quirks` | a backend response shape that differs from what the contract suggests |

## Skip

- typos and one-off fixes
- anything already written in AGENTS.md
- code structure, git history and file lists; they can be read again at any time
- facts that only matter for this conversation
- anything a single `grep` can rediscover

## Output

```text
LEARN
========================================================================
Written    <path | none>
Index      <line added | none>

-- Candidates ----------------------------------------------------------
#     KIND              DECISION  PATTERN
1     <kind>            save      <pattern>
2     <kind>            skip      <pattern> (<reason>)
========================================================================
```

If nothing is worth saving, say so. An empty memory beats a noisy one.

## Product knowledge

The team's reviewed product knowledge is in `.ai/knowledge/`. Route each candidate:

- A product lesson (a flow, a contract, a trap, a decision and its reason) becomes one proposal, `.ai/knowledge/inbox/<yyyy-mm-dd>-<slug>.md`, in the format of the `continuous-learning` skill. Never edit the pages; a human reviews the inbox.
- A standing instruction for agents is a rule, not knowledge: propose it for the project pack in `.ai/packs/`.
- After a feature with a spec ships, propose its `features/<slug>.md` page the same way, through the inbox: the PRD's goals and the TRD's final decisions (`D-n`, with accepted departures), each with code pointers. The spec stays as written; it is marked `superseded` only when a newer spec replaces it.
- Pages describe the main line. Tag a version-specific part in place, such as `(since 2.5.0)` or `(2.4 only)`; never a folder or page per version.

In the `LEARN` block, list the proposals in a `-- Proposed ---` section after `Candidates`, before the closing `=` rule, one path per row.
