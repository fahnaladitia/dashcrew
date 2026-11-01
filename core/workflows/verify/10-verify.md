---
target: workflow/verify
order: 10
rules:
  - verify.modes
  - verify.scope-from-base
  - skill.verify-phase
---
# verify

Run the phases in order. Stop at a phase marked STOP when it fails.

## Modes

- `quick`: phase 1 only.
- `full` (default): phases 1 to 5.
- `pre-pr`: phases 1 to 6.
- `codegen`: run the project's code generation step first, then `full`.

## Skills

- Apply `verification-loop`.

## Scope: the diff, not the repo

Every phase runs on the files this branch changed.
Run the whole repo only when the user asks or when cutting a release.

Determine the scope once, from the base branch chosen when the branch was created:

```bash
git diff --name-only <base>...HEAD
git status --porcelain | awk '{print $2}'
```

- The analyze list is both outputs combined, source and test files only: tracked changes against the base plus untracked files.
- The test list is every test file in that list, plus every test that imports a changed unit (`grep -rl "<file_name_without_ext>" <test folder>`).
