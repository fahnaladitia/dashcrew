---
name: checkpoint
description: "Record or compare a snapshot of the working state at a phase boundary, without writing anything to git."
argument-hint: "create <name> | verify <name> | list | clear"
---

# /checkpoint

Arguments: `create <name>` | `verify <name>` | `list` | `clear`.

## Hard limits

- Never run a git command that writes: no `commit`, `stash`, `tag`, `reset`, `checkout`, `apply` or `push`.
- Use only these git reads: `git rev-parse HEAD`, `git branch --show-current`, `git diff`, `git diff --stat`, `git status --porcelain`.
- Store each checkpoint as plain files (`<name>.patch`, `<name>.status`, `<name>.meta`) in a folder git ignores, for example `.ai/checkpoints/`.
- Nothing a checkpoint stores may enter the git object database.
- `git diff` sees tracked files only. Untracked files are listed in `<name>.status`, but their content is not saved.

## Restoring

- This workflow never restores anything. Restoring overwrites work in progress, and that is the user's decision.
- When the user wants to restore, give them the commands to run themselves: `git apply --check <dir>/<name>.patch` first, then `git apply <dir>/<name>.patch`.

## When to take one

Take checkpoints at phase boundaries, not every few minutes:

```
task start         -> /checkpoint create <task>-start
data layer green   -> /checkpoint create <task>-data-green
UI ready           -> /checkpoint create <task>-ui-ready
before refactor    -> /checkpoint create <task>-pre-refactor
after refactor     -> /checkpoint verify <task>-pre-refactor
```

A checkpoint is a session aid. It is not a backup and it does not replace a commit.
When the work must be safe, recommend a commit to the user.

## create

```bash
NAME="<name>"
DIR=".ai/checkpoints"
mkdir -p "$DIR"

git diff > "$DIR/$NAME.patch"
git status --porcelain > "$DIR/$NAME.status"

{
  echo "name:     $NAME"
  echo "time:     $(date +%Y-%m-%d-%H:%M)"
  echo "branch:   $(git branch --show-current)"
  echo "head:     $(git rev-parse --short HEAD)"
} > "$DIR/$NAME.meta"
```

Then run `flutter analyze` scoped to the folders and files the task is about to touch.
Never analyze the whole repo here.
Append the scope and the result to the meta file:

```bash
flutter analyze lib/ui/orders test/order_mapper_test.dart 2>&1 | tail -5
{
  echo "scope:    <folders and files analyzed>"
  echo "analyze:  <OK | N errors>"
} >> "$DIR/$NAME.meta"
```

- A checkpoint without an analyze result is useless as a baseline; always record it.
- Run no `flutter test` in `create`. The test baseline comes from `/verify` at step 5, after code exists.

## verify

Read the meta file, then compare the current state:

```bash
DIR=".ai/checkpoints"
NAME="<name>"

cat "$DIR/$NAME.meta"
git diff --stat
diff <(git status --porcelain) "$DIR/$NAME.status"
```

- Do not rerun `/verify full`. Reuse the numbers step 5 just produced.
- Rerun `flutter analyze` only on the scope recorded in the meta file, so the comparison is like for like.

```
CHECKPOINT: <name>  (<time>)

Branch:   <then> -> <now>
HEAD:     <then> -> <now>
Scope:    <from meta>
Analyze:  <then> -> <now>
Tests:    <X/Y passed, from /verify at step 5, scoped>
New files since checkpoint: <list>

Direction: [BETTER / SAME / WORSE]
```

- If the branch or HEAD changed since the checkpoint, say so first; a comparison across different bases can mislead.
- If the direction is WORSE, name exactly what broke, with `file:line`. "Something broke" is not a report.

## list

```bash
for f in .ai/checkpoints/*.meta; do echo "--- $f"; cat "$f"; done
```

## clear

Keep the 5 most recent checkpoints and delete the rest:

```bash
ls -t .ai/checkpoints/*.meta | tail -n +6 | while read -r m; do
  rm -f "${m%.meta}.meta" "${m%.meta}.patch" "${m%.meta}.status"
done
```
