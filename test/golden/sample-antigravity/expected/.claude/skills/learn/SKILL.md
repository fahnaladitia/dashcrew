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

```
LEARN

Candidates:
1. [<kind>] <pattern>  -> save / skip (<reason>)
2. ...

Written: <path>
Index:   <line added>
```

If nothing is worth saving, say so. An empty memory beats a noisy one.
