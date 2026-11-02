---
name: acceptance-criteria
description: "Write checkable Given/When/Then success criteria, with error states and their recovery, each mapped to the test that proves it. Use when: At step 0 when the task has no checkable criteria in a file, the prompt or the tracker task."
---

# Acceptance criteria

The criteria are the goal of the task: the step 0 block confirms them, the plan carries them, and the `reviewer` checks each one against the diff.

## Before writing

- Search the codebase and existing plans for the concept; features are often half built.
- Put every question that changes scope on the step 0 `Questions:` line. Record every other uncertainty as an assumption.

## Shape

- One behavior per criterion, as `AC-<n> Given <state>, when <action>, then <observable result>`.
- Group them: happy path, edge cases, error states.
- Every error state names its recovery: what the user sees when a request fails, and what they can do next.
- Non-functional criteria (offline, slow network, small screens, speed) only when the user asks for them.
- Rewrite subjective words ("fast", "easy") as something observable. QA must be able to read each criterion as one test without interpretation.
- Map each criterion to the test type that proves it, or to `manual`.

## From a business brief

When the request is a brief rather than a task, state before the criteria:

- the problem in one or two sentences, from the user's point of view;
- the stories, as `As a <role>, I want <action>, so that <benefit>`: one user value per story, and technical tasks are not stories;
- a concrete "not doing" list ("future enhancements" is not an entry);
- dependencies: backend contracts, design links the user gave, other features sharing the same repository or model;
- analytics candidates, as candidates only.

## Never

- Invent UI copy; copy comes from the design source.
- Add features the user might also want; they go on the "not doing" list or nowhere.
- Add roadmaps, rollout phases or business KPIs the app cannot measure, unless the user asked.
- Decide analytics events; list candidates and ask.
- Decide structure (state holder type, folder layout, identifier names); that is `feature-architecture`.

## Output

```text
AC-1  Given <state>, when <action>, then <result>     -> unit
AC-2  Given <request fails>, when <action>, then <message and retry>   -> ui
AC-3  Given <device offline>, when <action>, then <result>   -> manual
```
