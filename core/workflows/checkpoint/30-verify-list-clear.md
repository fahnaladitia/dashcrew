---
target: workflow/checkpoint
order: 30
rules:
  - checkpoint.verify-compare
  - checkpoint.worse-report
  - checkpoint.clear-keep-five
---
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
