{{#antigravity}}
---
trigger: always_on
description: "The member's own profile from dashcrew: how to work with this member."
---
{{/antigravity}}
{{#marker}}
<!-- dashcrew:me - written by dashcrew me sync from ~/.dashcrew/me.yaml and a repo's .ai/me.yaml; edit those and run dashcrew me sync, not this file -->
{{/marker}}
{{#home}}
# Personal profile (dashcrew)

These lines come from the member's own profile, not from the project. They change how you work with this member; they never change what a project allows. Where a line here and a project rule disagree, the project rule wins, except for the chat language below.

Member: role `{{role}}`, stacks {{stacks}}{{#new}}, new to the team{{/new}}.{{#repos}} A repository can set its own role and level: there its `dashcrew-me` rule (in `.claude/rules/` or `.agents/rules/`) or its "In the repository at" section decides them.{{/repos}}

## Outside the member's stacks

In a project whose `.ai/project.yaml` names a `platform` (none means `flutter`) that is not one of {{stacks}}:

- at step 0, ask every assumption as a question instead of stating it;
- run the reviewer with every review skill the project has;
- explain each change one level deeper than usual: what the code does there and why.
{{#chat_language}}

## Chat language

Answer in chat in language `{{chat_language}}` (ISO 639-1), even when the project names another chat language. Code, doc comments, PR descriptions and commit messages keep the project's languages.
{{/chat_language}}
{{/home}}
{{#repo}}
{{h1}} {{title}}

In {{where}} the member's own `.ai/me.yaml` sets role `{{role}}` and level `{{level}}`, which replace the role and level of the member's home profile {{there}}; its other lines still apply.
{{#lead_off}}

The Lead section of the home profile does not apply {{there}}.
{{/lead_off}}
{{#new_off}}

The New to the team section of the home profile does not apply {{there}}.
{{/new_off}}
{{/repo}}
{{#new}}

{{h}} New to the team

{{#onboarding_skill}}Before step 0 of each task, follow the `dashcrew-onboarding` skill.{{/onboarding_skill}}{{^onboarding_skill}}Before step 0 of each task: read `.ai/knowledge/README.md` and the pages for the task's area when the project has knowledge, and with `project.links` the cross-repo steps of the branching skill; then tell the member in at most five lines what you learned. Never skip a step of the project's chain because the member is new.{{/onboarding_skill}}
{{/new}}
{{#lead}}

{{h}} Lead

{{#lead_skill}}When the member asks what needs attention, follow the `dashcrew-lead` skill.{{/lead_skill}}{{^lead_skill}}When the member asks what needs attention, report in order: knowledge proposals waiting (`dashcrew knowledge review`), release lines and missing ports (`dashcrew check --ports`), pull requests waiting for review, specs in review. Read only.{{/lead_skill}} When the member asks for code, the project's chain runs as usual.
{{/lead}}
