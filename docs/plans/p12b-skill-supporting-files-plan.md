# P12b: supporting files travel with a project skill

Tier: standard, close to large. A project skill writes several files per agent; `catalog`, `adapters`, `sync`, `me`, `projectskill` and the goldens change. Follow-up from `docs/plans/p12-member-skills-plan.md` (Review round, not fixed by design).

## Problem

A skill folder often carries more than `SKILL.md`: `references/checklist.md` the body links to, a `scripts/check.sh` it runs, a template. Today only `SKILL.md` reaches the agents. `dashcrew skill adopt` copies only `SKILL.md` and names the files it left behind, so a body that says "see references/checklist.md" points at nothing.

## Design

- **catalog**: a project skill loads every other file of `.ai/skills/<name>/` (not `SKILL.md`, not `meta.yaml`) into `ProjectSkill.Files`, path relative to the skill folder, content verbatim, with an executable flag from the file mode.
- **adapters**: `SkillDoc` gains `Files`; each adapter writes them next to its `SKILL.md` (`.claude/skills/<name>/references/checklist.md`, `.agents/skills/<name>/...`). Catalog and pack skills have none, so their output does not change.
- **sync and lock**: the files are adapter files, so the lock records them and `doctor` drift and stale cover them with no new code. `sync` marks the executable ones executable after writing (Q1).
- **me sync** (optional skills): writes the same files into the member's clone. A file other than `SKILL.md` cannot hold the marker, so me sync also writes `.dashcrew-me` in the skill folder listing every file it wrote; turning the skill off removes exactly those files, then empty folders. A file in that folder that me sync did not write is kept.
- **skill adopt**: copies the whole folder, applying the limits below; it no longer reports skipped files except those the limits refuse.

## Limits (Q2)

- No symlinks; a file outside the folder is never followed.
- Hidden files and folders (`.DS_Store`, `.git/`) are skipped.
- At most 50 files and 1 MiB per file; over that, catalog and adopt refuse with the path.

## Answered

- Q1: the source's executable bit is kept: a `+x` file in `.ai/skills` is written `+x` for every agent.
- Q2: the limits stand: no symlinks, hidden files skipped, at most 50 files and 1 MiB per file.

## Tests (test first per stage)

1. `internal/catalog`: files loaded with paths and the executable flag; symlink, hidden file, count and size limits.
2. `internal/adapters`: every adapter writes the files next to `SKILL.md`; a catalog skill writes none.
3. `internal/sync`: an executable file is written executable and recorded in the lock.
4. `internal/me`: an optional skill's files written with `.dashcrew-me`; turned off, exactly those files go and a foreign file stays.
5. `internal/projectskill`: adopt copies the folder; refused files named.
6. Golden: `sample-project-skills` gains `references/checklist.md` and an executable `scripts/check.sh` on `api-style`, and a reference on the optional `ui-slop`; transcripts follow.
7. `go vet ./...`, `go test ./...`; no other sample changes, AGENTS.md unchanged.

## Not doing

- Supporting files for catalog or pack skills.
- Rendering supporting files as Mustache.
- Files from a URL.

## Review round

Ten findings; fixed with a test unless noted:

- A manifest line outside the skill folder (`..`, an absolute path) is ignored, and empty-folder pruning stops at the skill folder: a crafted `.dashcrew-me` cannot make `me sync` remove another file.
- `sync` sets execute bits only on supporting files it wrote: a file held by a release line is skipped instead of failing the run (no dedicated test; the modes now come from the rendered files, not a second list).
- Execute bits follow the source both ways, in `sync`, `me sync` and `adopt`, through one helper, `internal/filemode`.
- `me sync` status reports a manifest file the skill no longer carries ("left over") and a wrong execute bit ("stale"); `me sync` removes in sorted order and reports only files it removed.
- A pipe, socket or device in a skill folder is refused instead of read.
- `adopt` names hidden files at any depth and removes a half-written skill folder when a write fails (no dedicated test: the failure needs injection).
- Then by decision (Q2), changed in P12c (`docs/plans/p12c-skill-limits-and-me-load-plan.md`): a skill folder over the limits stopped every command that loads `.ai/`; it now skips the file with a warning.
