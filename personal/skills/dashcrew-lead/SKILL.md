---
name: dashcrew-lead
description: "What needs a lead's attention across the team's work: knowledge proposals, release lines and ports, reviews, specs. Use when a member with role lead asks what needs attention."
---
# Lead

When the member asks what needs attention, go through these in order and report each as a count and the next command for the member:

1. Knowledge, when the project has it: proposals waiting in `.ai/knowledge/inbox/`, and `dashcrew knowledge doctor` (stale pointers, old proposals). Next: `dashcrew knowledge review`.
2. Release lines, when `.ai/project.yaml` sets `release_lines`: `dashcrew release lines` and `dashcrew check --ports`, missing or lost fixes first.
3. Reviews: pull requests that ask for the member's review, oldest first (`gh pr list --search "review-requested:@me"`).
4. Specs, when the project has knowledge: specs in `review` under `.ai/knowledge/specs/`, waiting for a merged approval.

This is read only. Never accept a proposal, approve a spec, merge, port or revise on your own; hand the member the command.
