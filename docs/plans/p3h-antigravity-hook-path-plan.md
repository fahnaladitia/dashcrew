# P3h: the Antigravity guard hook finds the shim from any working directory

Tier: standard. The Antigravity hook command and matcher change (generated output in three samples), the guard answers Antigravity on every allowed call and reads more write tools (the guard corpus record gains allow lines), and `doctor` asks agy for its active hooks.

## Problem

`.agents/hooks.json` runs the guard as `./.ai/bin/guard`. The member's Antigravity (the `agy` CLI) looks for it in `.agents/.ai/bin/guard`, which does not exist, so the guard does not run.

P3c chose the relative command because the public docs left the working directory and the shell open; it named this as the remaining risk ("if Antigravity runs hooks from another directory, the guard does not run"). The hooks guide bundled in `agy` (the `docs/hooks.md` text in the binary) now settles both:

- "`command` (string, required): The shell command to execute (run via `sh -c` on Unix, `cmd /c` on Windows)."
- "The working directory is set to the directory containing `hooks.json`."

So for the workspace file `<repo>/.agents/hooks.json` the working directory is `<repo>/.agents/`, and `./.ai/bin/guard` points one folder too deep. Older `agy` builds seem to have run hooks from the workspace root (the bundled changelog fixes workspace hook loading more than once), which is why it worked some of the time.

## Change (confirmed: the git form, as Codex)

- The hook command becomes the Codex one:
  `"$(git rev-parse --show-toplevel)/.ai/bin/guard" "$(git rev-parse --show-toplevel)/.ai/guard.json"`.
  It finds the shim from `.agents/`, from the root, or from any other folder of the repo, and it passes the policy, so the guard no longer depends on `workspacePaths[0]` (which is a parent folder when the member opens one).
- One constant for both adapters instead of two strings.
- `doctor` `antigravity-hook` already compares the key with what sync writes: an existing project fails it until `dashcrew sync` rewrites the key, which `Merge` does in place.
- README (Antigravity rows) and `docs/plans/p3c-antigravity-plan.md` record the settled behavior.

## Limits

- Windows: `agy` runs hooks with `cmd /c`, which has no `$(...)`. The shim is an `sh` script, so the current command does not run there either; this change neither fixes nor breaks Windows.
- A repo that is not a git checkout gets no shim path. dashcrew projects are git checkouts, and Codex has the same requirement.

## Tests (test first)

1. `internal/adapters`: the Antigravity hook command is the shared git form with the policy argument.
2. `internal/doctor`: a project synced with the old relative command fails `antigravity-hook`; a resync passes.
3. `internal/guard`: an Antigravity payload with the policy argument and `workspacePaths` pointing at a parent folder still blocks `git push`.
4. Goldens: `.agents/hooks.json` in `sample-antigravity`, `sample-knowledge` and `sample-other`, their lock entries and transcripts.
5. `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`.

## Not doing

- A Windows hook command (`cmd /c`): the shim would need a `.cmd` twin first.
- Changing the Claude Code or Codex hooks.

## Findings from the Antigravity docs (2026-11-04)

Sources: the hooks guide and changelog bundled in the member's `agy` 1.2.14 binary, and https://antigravity.google/docs/hooks?tab=cli.

Why the working directory is `.agents/`: a hook command resolves against the folder of the `hooks.json` that declares it, so a plugin's or the global file's `./scripts/x.sh` works wherever it is installed. The docs' `./scripts/lint.sh` example means `.agents/scripts/lint.sh`, not a path from the project root; P3c read it the other way.

Further gaps, by severity:

1. No decision for an allowed call. The guard prints nothing when it allows; the docs mark `decision` as required. Third-party integrations report that agy fails closed on a missing decision, and the 1.2.14 changelog fixed an "unknown pre-tool hook decision" error on an empty decision, so older builds failed. Once the path is fixed, an older agy could block every allowed call. In the current docs `allow` "allows the tool call to proceed, delegating to the next hook or default policy" (auto-approval is `auto_approve`), so emitting `{"decision": "allow"}` no longer changes the member's permission flow, which was P3c's reason to emit nothing.
2. Write tools the matcher misses. agy's own permission vetting covers `create_file`, `delete_file`, `edit_file`, `file_delete`, `file_write`, `str_replace_editor`, `write_file` besides the three dashcrew matches, and the binary has a `git_commit` step type. A write through one of them never reaches the guard. Their argument shapes are not documented; finding them needs a logging hook in a live agy.
3. Workspace hooks load only in a trusted folder (1.2.14 changelog: "workspace-local hooks ... not loading after trusting a folder"). In an untrusted folder the guard is silently absent, and doctor cannot see it today; `agy -p /hooks` prints the active hooks non-interactively and could back a doctor check.
4. Several named hooks for one event run in sequence and a later one may `overwrite` the tool arguments after the guard checked them; dashcrew cannot order other hooks.
5. Windows: `cmd /c` cannot run the `sh` shim, so the guard never runs under Antigravity on Windows.

## Scope after the findings (confirmed)

P3h also takes gaps 1, 2 and 3; 4 and 5 stay out.

- Gap 1: for an Antigravity payload the guard answers an allowed call with `{"decision": "allow"}` (exit 0), and the shim's success path stays the guard's.
- Gap 3: README tells Antigravity users to trust the folder. When `agy` is on `PATH`, `doctor` `antigravity-hook` runs `agy -p /hooks` and warns when `dashcrew-guard` is not among the active hooks (an untrusted folder, a budget drop); without `agy` it keeps today's file check.
- Gap 2 (confirmed: conservative, no live capture, since the maintainer has no working agy): the matcher covers the ten write tools of agy's permission vetting (`write_to_file`, `replace_file_content`, `multi_replace_file_content`, `create_file`, `edit_file`, `write_file`, `file_write`, `str_replace_editor`, `delete_file`, `file_delete`). For a tool other than the documented three, the guard reads the path from the first of `TargetFile`, `AbsolutePath`, `file_path`, `path`, and the new text from every present one of `CodeContent`, `content`, `file_text`, `new_str`, `ReplacementChunks`; it checks that as a write (or as a delete when the tool name says so) to that path. A call with none of the path keys is allowed, as an unknown tool is today. Marked unverified until a capture.

### Capture recipe (for a member with agy)

In a scratch git repo, `.agents/hooks.json`:

```json
{ "dashcrew-probe": { "PreToolUse": [ { "matcher": "", "hooks": [ { "type": "command", "command": "./log-hook.sh", "timeout": 10 } ] } ] } }
```

and an executable `.agents/log-hook.sh` that appends `{"cwd": "<pwd>", "payload": <stdin>}` to a log and prints `{"decision":"allow"}`. Trust the folder, ask agy to create, edit, delete and rename files, run `git status` and commit, then repeat under another model. The log gives each tool's name and arguments, and its `cwd` lines confirm the `.agents/` working directory.

## Review round

- Fixed with a test: `edit_file`-style arguments (`target_file`, `filePath`, `file`, `code_edit`) are read; `str_replace_editor` with `command: view` is a read, not a write; doctor reads `agy -p /hooks` record by record, treats a skipped guard or a failure that names trust as not loaded, and sets `WaitDelay` so a child holding the pipes cannot hang it; the policy argument holds when the workspace is a parent folder.
- Shared helpers: `replacementTexts` for both multi-chunk readers, `Doctor.searchPath` for the three PATH lookups.

Decided after the review:

- The hook command tries `..` (the folder above `.agents/`), then `.`, then the git toplevel, and runs the first shim it finds with that folder's policy; with none it prints a deny decision naming `dashcrew sync`. It serves a project below the git toplevel and a checkout without git, which the git-only form missed. Codex keeps `GitGuardCommand`.
- `version.AntigravityGuardMinimum` is 0.5.0: sync writes it into the shim and the lock of a project with the antigravity agent, so an older dashcrew, which sends no allow decision and reads none of the new write tools, is refused with the install hint.
- `allow` stays: safe-chains tested agy 1.1.2 and found that `allow` does not skip agy's own confirmation, and the current docs say it hands the call on.
