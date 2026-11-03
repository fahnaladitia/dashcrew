# P12c: skill limits warn, me sync reads only .ai/skills

Tier: standard. `catalog`, `sync`, `doctor`, `me`, `projectskill`; generated output unchanged. The two follow-ups left in `docs/plans/p12-member-skills-plan.md` and `docs/plans/p12b-skill-supporting-files-plan.md`. The maintainer asked to do both directly.

## 1. A file over the limits is skipped with a warning

Today a symlink, a pipe, socket or device, a file over 1 MiB, or a 51st file in `.ai/skills/<name>/` stops every command that loads `.ai/` (`sync`, `doctor`, `worktree`, `link`, `me sync`, `adopt`).

- `catalog`: the file is left out and recorded in `ProjectSkill.Skipped` as `<path>: <reason>`; the skill loads with the files that pass. Files are taken in path order, so the first 50 stay. An I/O error still fails.
- `sync` prints one warning per skipped file; `doctor` `skills` warns with them; `me sync` reports them for an optional skill it writes; `skill adopt` leaves them out and names them with the hidden files.
- Unchanged: a bad `meta.yaml`, a missing `SKILL.md`, a taken name, `review` with `optional` still fail, since the skill itself is wrong.

## 2. me sync reads only `.ai/skills`

Today `me sync` and status call `WithProjectPacks` per registered repo, parsing every project pack and platform only to read `.ai/skills`.

- `catalog.LoadProjectSkills(projectDir, taken)` reads `.ai/skills` alone. `me` passes the shipped catalog's taken names, computed once per run. A clash with a project pack's skill name is still caught by `sync` and `doctor`, which load the whole `.ai/`.

## Tests (test first)

1. `internal/catalog`: the limit cases load with `Skipped`; the 51st file is the one skipped; a FIFO is skipped, not read; `LoadProjectSkills` reads skills without project packs.
2. `internal/sync`: a skipped file becomes a warning and is not written.
3. `internal/doctor`: `skills` warns with the skipped file.
4. `internal/me`: a broken project pack no longer stops the repo's optional skills.
5. `internal/projectskill`: adopt skips a symlink and names it.
6. `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`; goldens change only in the `skill-adopt.txt` line that names the hidden file left out.

## Review round

Nine findings, each fixed with a test:

- A symlinked `SKILL.md` or `meta.yaml` is fatal again: it would copy the link's target into every agent's skill. The other limits still only skip.
- `me sync` adds the names the repo's own packs and platforms take, read from their folder layout, so it never writes an optional skill `sync` refuses.
- Files past the first 50 are one entry, `<dir>: N files past the first 50 (first five...)`, and the 50 kept are the first by full path, as documented.
- `doctor` names the files left out in its `Fail` line too, not only when nothing else fails.
- `sync` and `doctor` warn only for required skills; `me sync` reports the optional ones it writes.
- One wording, `ProjectSkill.Warnings`, for sync, doctor and me sync; reasons carry no `;`, so doctor's `; `-joined line splits cleanly.
- `skill adopt` prints one `left out  <path>: <reason>` line per file, hidden ones included.
