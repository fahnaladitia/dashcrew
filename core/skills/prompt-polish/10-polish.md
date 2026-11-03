---
target: skill/prompt-polish
order: 10
rules: [workflow.step0-block, workflow.step0-idea-refine, workflow.step0-prompt-template-file]
---
# Prompt polish (step 0)

Turn the request into the step 0 block below, print it in chat, and wait for the user to confirm it. If the project keeps its own prompt template, fill that one; it wins over this one.

## Refine before writing

- Who is this for, and what core problem does it solve?
- Which assumptions hide in the request? Name each one; when the request reads two ways, name both.
- Keep scope tight: what is left out goes on the `Not doing` row. Use the `idea-refine` skill when it is installed.

## Block

```text
STEP 0  <feat | fix | hotfix>: <polished request, one sentence>
========================================================================
Scope      <directories and files>
Not doing  <what stays out | none>
Tier       <trivial | small | standard | large>  (<evidence>)
{{#has_release_lines}}
Release    <version> | none
{{/has_release_lines}}
{{#has_knowledge}}
Spec       specs/<release>/<feature>@rev<n> | none
{{/has_knowledge}}

-- Assumptions ---------------------------------------------------------
A1    <assumption>

-- Questions -----------------------------------------------------------
Q1    <question>

-- Skills --------------------------------------------------------------
NAME                 WHY
<name>               <why>

-- Success criteria ----------------------------------------------------
ID    CRITERION                                 TEST
<id>  <criterion>                               <test type>

-- Branch --------------------------------------------------------------
1     <name>
2     <name>
3     <name>
base  <ask>
========================================================================
Reply "ok", or answer by id (Q1 ...), pick a branch and a base, edit any line.
```

An empty list prints `none` under its rule. Columns line up with spaces; a long value wraps under its own column.

