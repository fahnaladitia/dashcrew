---
target: workflow/audit
order: 20
rules:
  - audit.path-required
  - audit.mechanical-checks
  - audit.parallel-reviewers
  - audit.split-large
  - audit.consolidate-findings
  - audit.not-findings
---
## Code mode: `audit <path> [--focus ...]`

- A path is required. Without one, ask.
- Audit all of `lib/` only when the user writes it explicitly; then audit it one feature folder at a time.
- An empty `--focus` means every focus.

### Phase 1: Scope

```bash
find <path> -name '*.dart' ! -name '*.g.dart' ! -name '*.freezed.dart' | wc -l
find <path> -name '*.dart' ! -name '*.g.dart' ! -name '*.freezed.dart'
```

Find the tests covering each mapper, repository, model and state holder in the path: `grep -rl "<file_name_without_ext>" test/`.

### Phase 2: Mechanical checks (main session)

Run these first:

```bash
flutter analyze <path> 2>&1 | tail -40
grep -rn "print(" <path> --include="*.dart" | grep -v debugPrint
grep -rn "http://\|badCertificateCallback" <path> --include="*.dart"
grep -rn "dynamic" <path> --include="*.dart" | grep -v "\.g\.dart"
grep -rln "CachedNetworkImage\|Image.network" <path> --include="*.dart" | xargs grep -L "memCacheWidth\|cacheWidth"
grep -rln "Controller(\|FocusNode(\|StreamSubscription\|Timer" <path> --include="*.dart" | xargs grep -L "dispose()\|cancel()"
```

Selected packs add their own checks to this list.

- A grep hit is only a candidate. Read every hit before it becomes a finding.
- `dynamic` inside a JSON mapper, or a timer cancelled when the state holder closes, is not a violation.
- When a code graph tool is connected, use it to find `build` methods over 100 lines and files many features depend on (risky to change later).

### Phase 3: Reviewers (parallel, read-only)

- Show the plan first: one line per focus with the reviewer it gets, or SKIP with evidence.
- Then start one `reviewer` per focus in one message.

| Focus | Who | Checklist |
|---|---|---|
| arch, lifecycle | `reviewer` | AGENTS.md architecture and Dart and Flutter rules |
| perf | `reviewer` (a second prompt) | `flutter-performance`; `performance-optimization` for the measure-first principle only |
| security | `reviewer` | `mobile-security` |
| qa | `reviewer` | `qa-identifiers` when the qa pack is selected, else the QA identifier rules in AGENTS.md |
| test | main session | Phase 1 results |

Every reviewer prompt says:

- audit the existing code in `<path>`, not a diff; report only, edit and write nothing
- run no `flutter test`
- the Phase 2 results relevant to its focus, so it does not search again
- for qa: audit identifiers only, write no tests
- for security: run any scanner from a scratch folder, then check `git status --porcelain` and move any stray output out of the repo

More than 80 files: split by subfolder, with at most 6 reviewers at once.

Extra checks for `arch` and `perf`:

- boolean flag soup in state (`isLoading`, `isError`, `hasData` as separate flags)
- a state holder depending directly on another state holder instead of a repository
- `_buildXxx()` helpers returning widgets instead of `const` widget classes
- `MediaQuery.of(context)` where `MediaQuery.sizeOf(context)` is enough
- `IntrinsicHeight` or `IntrinsicWidth` inside a scrolling list
- expensive work in `build` (sorting, filtering, regex)

### Phase 4: Consolidate

- Use the `code-review` severities: BLOCKER, HIGH, MEDIUM.
- Merge repeated findings into one ("7 widgets without `const` in `components/`" is one finding, not seven).
- Keep only findings you are sure of. In doubt, open the file again or drop the finding.
- Every finding has `file:line`, the current code and a concrete fix.
- Not findings in an audit: missing Dartdoc on old code, style, indentation, formatting.

### Output

```
AUDIT <path>: <n> files, focus <list>

Analyze:   <n> errors, <m> warnings
BLOCKER    n
HIGH       n
MEDIUM     n
Test gap:  <n> data-layer units without tests
QA ids:    <n> QA-relevant widgets without identifier

Top 3 actions:
1) [<focus>] <action> (<file>)
2) ...
3) ...

File: <audit file>
```
