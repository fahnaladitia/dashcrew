# P12: member skills (try, adopt, switch)

Tier: large. New project skill source, a `me.yaml` key, a CLI command, and changes in `catalog`, `render`, `sync`, `me`, `doctor` and the adapters; generated output gains files only in projects that use it. Goldens get one new sample.

## Problem

A member wants a skill dashcrew does not ship, such as an "anti UI slop" checklist. Today:

- Tried alone, it lives in `~/.claude/skills` outside dashcrew; step 0 picks only from the skill index plus a fixed list of outside skills, so the agent may never pick it.
- Adopted by the team, it needs a project pack and a made-up `stack` family just to carry one skill, and it is then on for every member.
- No member can turn a team skill on or off for themselves: `me.yaml` selects only the two personal skills dashcrew ships (`dashcrew-onboarding`, `dashcrew-lead`).

## Decisions (confirmed)

- A project skill says whether it is optional (`optional: true` in its meta). A required one is on for everyone. An optional one is off until a member turns it on.
- A skill with `review: true` is the team's review checklist and is never optional: the catalog refuses `review: true` with `optional: true`.
- The guard is untouched and never reads `me.yaml`; a member's switch only changes which skills step 0 may pick for that member.

## Flow

```text
-- Flow ----------------------------------------------------------------
#  STEP              WHERE                             WHO SEES IT
1  try alone         ~/.dashcrew/skills/<name>/        the member only
                     + me.yaml  skills: [<name>]
2  adopt for team    dashcrew skill adopt <name>       committed; every
                     -> .ai/skills/<name>/             member after sync
3  switch per member .ai/me.yaml or ~/.dashcrew/       the member only
                     me.yaml  skills: [<name>]
========================================================================
```

## Design

### Project skills: `.ai/skills/<name>/`

- `meta.yaml`: `description`, `when_to_use`, `review`, `optional` (default `false`).
- `SKILL.md`: the body, copied verbatim to every agent. It is not rendered as mustache, so an outside skill with `{{` survives unchanged; it cannot use catalog variables.
- A required project skill: `sync` writes it like a catalog skill (`.claude/skills/<name>/SKILL.md`, `.agents/skills/<name>/SKILL.md`), each adapter adding its own front matter, records it in the lock, and adds it to the AGENTS.md skill index.
- An optional project skill: `sync` writes nothing for it and it stays out of the index (AGENTS.md does not grow; the budget has 19 bytes left). `me sync` writes it into the same agent folders only in the clone of a member who turned it on, and lists those paths in `.git/info/exclude`, like `.claude/rules/dashcrew-me.md`. A member who did not turn it on has no file, so no agent can pick it, Claude Code's own skill discovery included. Task worktrees link these files like the other git-excluded toolkit paths.
- A project skill with the name of a catalog skill is an error.

### `me.yaml`: `skills`

- `skills: [<name>, ...]` in `~/.dashcrew/me.yaml` (every repo) and in a repo's `.ai/me.yaml` (that repo only, added to home).
- A name is either a personal skill under `~/.dashcrew/skills/<name>/` (step 1) or an optional project skill of the repo (step 3).
- `me sync`:
  - links each personal skill into the Claude Code skills folder, like `dashcrew-onboarding` today (Codex and Antigravity get none, as today);
  - writes each turned-on optional project skill into that repo's agent skill folders, git-excluded, and removes the ones turned off;
  - writes into the personal instructions, per repo, the skills this member may pick at step 0 beyond the index: each with its `when_to_use`.
- A required project skill or a catalog skill listed in `skills` is ignored with a `doctor` warning (it is on anyway). A name that is neither a personal skill nor an optional project skill is a `doctor` warning.

### `dashcrew skill adopt <name | folder> [--required]`

- Copies `~/.dashcrew/skills/<name>/SKILL.md`, or `<folder>/SKILL.md` (the skill is named after the folder), to `.ai/skills/<name>/`, writes `meta.yaml` from its front matter (`description`, `when_to_use`) with `optional: true`, or `false` with `--required`.
- Refuses a name taken by a catalog skill or an existing project skill. Never commits.
- Prints the next steps: run `dashcrew sync`, commit, and remove the personal copy (`doctor` warns while both exist).

### `doctor`

- `skills`: project skills installed and current.
- `me`: unknown names in `skills`, a personal skill shadowed by a project skill of the same name, a required or review skill listed in `skills`.

## Answered

- Q1: the command is `dashcrew skill adopt`, a new `skill` group, since the result is committed to the repo.
- Q2: `adopt` takes a personal skill name or a local folder (`dashcrew skill adopt ./anti-ui-slop`); no URL.

## Tests (test first per stage)

1. `internal/catalog`: load `.ai/skills/` (meta keys, verbatim body, `review` with `optional` refused, a name clash refused).
2. `internal/render`: a required project skill renders for every agent and joins the index; an optional one renders but stays out of the index; AGENTS.md budget unchanged.
3. `internal/me`: `skills` parsed in both layers; personal skills linked; the per-repo instructions list the turned-on optional skills; unknown names kept for doctor.
4. `internal/doctor`: the new warnings.
5. `cmd/dashcrew`: transcript `skill-adopt.txt` (adopt, sync, me sync, doctor).
6. Golden: new sample `sample-project-skills` with one required and one optional project skill.
7. `go vet ./...`, `go test ./...`; existing goldens unchanged except lock digests if any.

## Not doing

- Per-member switches for catalog or pack skills (they follow the repo's stack).
- Turning a required or review skill off for one member.
- Personal skills for Codex and Antigravity.
- Fetching skills from a URL or a registry.

## Review round

Ten findings, each fixed with a test unless noted:

- A repo whose `.ai/` does not load keeps its turned-on skill files; me sync reports it and leaves them.
- `me sync` status renders the home rules with the same own skills as `me sync`, so it is in step right after a sync.
- Turning a skill off, or making it required, drops its `.git/info/exclude` lines, so the files `sync` then writes show in `git status`.
- One front matter reader, `catalog.SkillFrontMatter`, parses YAML (quotes, folded scalars, nesting) for `adopt` and `me`.
- `doctor` fails on a missing required project skill, like a dashcrew skill.
- `me sync` unlinks only own skills it linked (`~/.dashcrew/me-linked-skills`), and never one still listed.
- `skill adopt` needs `.ai/project.yaml` in the project folder.
- Shared helpers: `catalog.ValidProjectSkillName`, `adapters.SkillFile` (the agent skill paths), `linkOrCopy`.
- Not fixed, by design: only `SKILL.md` reaches the agents. `adopt` lists the other files of the folder it did not carry.
- Not fixed: `me sync` overlays the project packs per repo to read `.ai/skills`; the shipped catalog is read once per run.
