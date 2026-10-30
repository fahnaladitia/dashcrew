---
target: workflow/checkpoint
order: 20
rules:
  - checkpoint.create-analyze-scoped
  - checkpoint.create-no-tests
---
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
