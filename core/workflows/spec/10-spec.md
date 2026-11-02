---
target: workflow/spec
order: 10
when: { has_knowledge: true }
rules:
  - spec.outside-chain
  - spec.knowledge-repo-only
  - spec.brainstorm-one-round
  - spec.prd-acceptance-criteria
  - spec.trd-feature-architecture
  - spec.ids-never-renumbered
  - spec.owners-are-roles
  - spec.ask-open-questions
  - spec.lint-before-handover
  - spec.member-opens-pr
  - spec.approval-is-merged-pr
---
# spec

The main session runs this workflow, outside the chain, for a feature large enough to need a PRD and a TRD before tasks start, often across several repos. The spec lives only in the knowledge repo: `.ai/knowledge/specs/<release>/<feature>/prd.md` and `trd.md`.

## Start

- `dashcrew spec new <feature> [--release <version>]` writes both files from the repo's templates with `status: draft` and `rev: 1`. The release defaults to the one the branch records, else `unreleased`.
- It refuses a folder that exists. The guard lets you write a spec file only while it is new, `draft` or `review`; an approved one is reopened by a member with `dashcrew spec revise <folder>`, never by you.

## 1. Brainstorm

Before writing, ask every question in one round: the problem, the users, two or three options with their trade-offs, the assumptions, and what is not done. Wait for the answers; what is still open becomes an assumption or an open question. There is no second brainstorm; questions that come up while writing are asked in step 4.

## 2. PRD

Fill `prd.md` with `acceptance-criteria`: problem, users, goals, scope, Not doing, then the criteria. Every criterion starts its list item with its ID, `AC-1`, `AC-2`, numbered once and never renumbered. A dropped one stays, struck through with its reason: `~~AC-3~~ dropped: <why>`.

## 3. TRD

Fill `trd.md` with `feature-architecture`, in these sections:

{{trd_sections}}

Contracts and decisions take IDs under the same rules as criteria. Every open question ends with `owner: <role>`. Owners, in the front matter and on questions, are roles such as `backend` or `mobile`, never names or emails.

## 4. Questions

Before lint, show every open question and every assumption the PRD and TRD gained since the brainstorm, as one list, and ask the user once. An answer becomes a decision (`D-n`) or a criterion and leaves the open questions; only what the user cannot settle, another role's call, stays with `owner: <role>`. Never hand over a spec with a question the user could have answered.

## 5. Lint

Run `dashcrew spec lint .ai/knowledge/specs/<release>/<feature>` and fix every problem until it exits 0.

## 6. Hand over

Set `status: review` in both files, then print these commands for the member and never run them:

```bash
git -C .ai/knowledge checkout -b spec-<feature>
git -C .ai/knowledge add specs/<release>/<feature>
git -C .ai/knowledge commit -m "spec: <feature> for <release>"
git -C .ai/knowledge push -u origin spec-<feature>
```

The member opens the pull request on the knowledge repo. Approval is its merge, with `status: approved` set by the member in that pull request; there is no approve command.
