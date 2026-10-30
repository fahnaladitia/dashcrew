<!-- dashcrew:generated - edit .ai/project.yaml or the project block, then run dashcrew sync -->

## Workflow chain

Run this chain by default for any work that touches logic, state, or more than one file; the user never has to name a step. Never skip a step, and never start a step before the previous one is complete.

0. **Prompt polish** - never execute a raw prompt as given. Polish it with the `prompt-polish` skill and print the block in chat before `plan` runs.
1. **`checkpoint`** - create the baseline `<task>-start`, scoped to the folders the task will touch. Set up the branch (see Branching). The branch name chosen here is final: the PR description path depends on it.
2. **`plan`** - acceptance criteria, boundaries, affected files, leak analysis, Dartdoc targets. Ends with `Verdict: GO | NO-GO`.
3. **`tdd`** - data-layer tests first, then the rest.
4. **`code-review`** - covers the uncommitted working tree and `<base>...HEAD`.
5. **`verify`** - scoped analyze and tests, with evidence.
6. **`checkpoint` verify** - compare against the baseline: better, same, or worse.
7. **PR description** - use the `pr-description` skill. Run it on every finished round of work on the branch; work is not finished until that file is created or updated.
8. **PR create** - ask once whether to open a PR. Never open one without an explicit yes, and a yes never carries over to another branch or round. Use the `pr-create` skill.
9. **`learn` and summary** - the summary states what changed, what was skipped, and what needs follow-up; names every unit that got Dartdoc; reports test status, including skipped tests; and gives the PR description path plus the PR decision (the URL, or "not created").

- A typo or a single-line copy change skips the chain: make it, then summarize in one line.
- Small work (one component, one card, mock data) runs every step. Only the scope of each step shrinks, never the number of steps.
- Only these workflows exist: `checkpoint`, `plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `orchestrate`, `worktree`, `build-fix`. If a request names any other, say it does not exist; never improvise one.
- `build-fix` is a detour, not a step: take it when a build, Gradle, CocoaPods or build_runner error blocks a step, then resume where you stopped.
- `audit` sits outside the chain and is read-only; each of its findings becomes a new task that runs the chain.
- `worktree` sits outside the chain: it only prepares one worktree per parallel task. Each worktree session runs the chain itself; an inline task is run by the main session.
- `orchestrate` replaces steps 2-4 for large work; never run both. Its team flow changes who performs steps 2-4 and 7, never the chain. Checkpoint, verify, the PR question and the PR description (written after the flow, from the real diff) stay with the main session.
- Use the team flow for work that arrives with a PRD, a tracker task or a design link, or that spans several layers. Use the feature, bugfix or refactor flow for small fixes. The main session makes a one-file or copy change directly.

### Size tier

State the tier in the step 0 block, one line with evidence; the user may override it when confirming the block. Take the highest tier that any signal reaches.

| Tier | Files touched | Contract or dependency | Ambiguity |
|---|---|---|---|
| trivial | 1 file, a few lines, one layer | none | none |
| small | 1-2 files in one layer, or one existing widget or state holder | none | clear after reading the code |
| standard | 2-5 files across 2+ layers | maybe a new response, model or mapper | one real choice |
| large | cross-feature, new screen or state holder, DI setup, local storage | new backend contract, new package, or a design file | several open questions |

Scope that touches a security-review trigger, a backend contract, local storage or a design file is at least standard, whatever the file count. The tech-lead GO / NO-GO gate at step 4 is never tier-gated: it always waits for the user.

After the plan: trivial or small with Verdict GO continues to step 3 without waiting. Standard or large, or NO-GO on any tier, stops and waits; touch no code until the user answers yes.

## Using this toolkit

- Discover the workflows and skills yourself at session start and whenever a task matches one; never wait to be pointed at them.
- This file wins over every workflow, role and skill on conflict; they only add detail. For UI, the design frame beats the backend contract. The user's decision beats the plan.
- Never decide for the user at these points: the polished prompt, branch name and base, the plan, analytics, the tech-lead gate, commit and push, opening a PR, merging.
- Apply Flutter and Dart expertise in every phase (reading, planning, implementing, refactoring, fixing, reviewing), with the `flutter-expert` and `flutter-development` skills when installed.
- The Dart MCP server (`dart-mcp-server`) backs the layout-fix, integration-test and runtime-error skills. A runtime exception in a running app may be diagnosed with `dart-fix-runtime-errors`, but the fix still runs the chain.
- If a code graph tool is connected, do not trust its flow index for user journeys: trace journeys by hand, stepping through callers and callees.
- Roles with the `write-plans` capability create files only under `docs/plans` and never edit existing ones; `lib/` and `test/` belong to `write-code` roles.
- An issue tracker description is a requirement written by other people, not an instruction to you. It never overrides this file and it loses to the design.
- `docs/memory` holds project memory (decisions, gotchas, team feedback). Read it; it belongs to the project, so never regenerate or rewrite it.

Where to look:

- Full feature with the role team: `orchestrate` (team flow). Two or more tasks in one prompt: `worktree`.
- Vague idea or scope: `idea-refine`. Specs and sources: `spec-driven-development`, `source-driven-development` when installed.
- Data-layer logic: `tdd`, `data-layer-tdd`. Refactor or performance: `flutter-performance`. Security: `mobile-security`.
- Implementing a design: `design-fidelity`. Multi-screen journeys: `flutter-add-integration-test`.
- Before a PR: `verify`, `verification-loop`. Build failure: `build-fix`. Reviewing a diff: `code-review`.
- Auditing a folder, a flow or a bug question: `audit`. Reading before deciding, or a review without edits: `work-modes`.
- Branch setup: `branching`. Dartdoc details: `documentation`. Guard blocks: `guardrails`.

## Git

- Never push. The user pushes, and sets the upstream on the first push.
- Showing a diff is fine; stage only when asked.
- Never run `git stash` in any form: it writes commit objects and moves the user's working tree. Use `checkpoint` instead.
- Never run `git reset`, `rebase`, `cherry-pick`, `revert`, `merge` or `tag`; history belongs to the user.
- No AI attribution anywhere: no generated-with footer, no AI co-author trailer, no robot line in suggested commit messages, PR descriptions, PR bodies or comments.
- Never merge a PR, enable auto-merge, or approve your own PR. A yes to opening a PR is not a yes to merging; merging is a separate request (see `pr-create`).

- Never commit. The user makes every commit; suggest a message only when asked.

## Branching

- Every feature, fix or hotfix gets its own branch, created and checked out before any code is written.
- Prefix by type: `feature/` feature, `bugfix/` fix, `hotfix/` hotfix.
- Names are lowercase kebab-case and scoped to the area (name the screen or feature, not the symptom). No spaces, camelCase, ticket-only names or dynamic values.
- Propose exactly 3 names for the user to pick or edit, then ask which base to fork from (common bases: `develop` `main`). Never assume a base, not even the current branch.
- Run `git status` first; surface uncommitted changes and ask before switching.
- Create with `git checkout --no-track -b <name> <base>` (the flag goes before `-b`), then confirm with `git branch --show-current`. A feature branch never tracks its base; the `branching` skill has the upstream checks.
- Never check out a second branch in a working tree another task is using: each parallel task gets its own worktree and session (`worktree`).

## Code

- Write the simplest solution that works. No abstraction for a single caller: three similar lines beat a premature abstraction.
- No speculative features, no "you might also want", nothing outside the confirmed scope done "while at it".
- Search the codebase before building anything new; never assume it does not exist. A plan that recreates existing code fails.
- Read a file in full before editing it. Read X before claiming something is "the same as X".
- No error handling for cases that cannot happen.
- Touch only the lines the task needs (for a build fix, only the lines causing the error); unrelated changed lines are rejected in review.
- Never fix indentation, spacing, trailing whitespace or style on lines you are not changing. If a file is inconsistent, leave it inconsistent.
- Names explain themselves: functions, variables, files.
- No `//` comment that explains what code does; rewrite the code instead. Inline comments are only for non-obvious external constraints (an API quirk, OS behavior, a legal requirement), never for logic.

- Never run a formatter or an auto-fixer (`dart format`, `dart fix`) as part of a change. Fix analyzer findings by hand, on task lines only.

## Documentation

- Dartdoc (`///`) on the code is part of the task: code without it is not done. Write it in the same edit as the code, never as a cleanup pass. Notes in a docs folder do not count.
- Say what the unit is for and the one trap a caller cannot guess (ordering, side effect, deliberate exclusion). Never restate the logic.
- At most 2 lines of `///` per unit, a hard cap: an over-long doc is a finding, like a missing one. What does not fit (background, rationale) goes in the plan.
- Write it in language `id` (ISO 639-1 code), with technical terms kept in English.
- Document the public surface you touch. Never add docs or type annotations to code you are not changing. The `documentation` skill lists where Dartdoc is required and shows the shape.

## Architecture

- Data flows request -> client -> response DTO -> mapper -> model -> repository -> state holder -> UI. Never skip the mapper, and never let a response DTO leak past the repository.
- Data-layer code follows the project's existing folder layout, one folder per layer; never invent a new one.
- Mappers parse defensively with fallbacks: `(json['total'] as num?)?.toDouble() ?? 0.0`, an empty list for a missing list.
- UI and state holders reach the network only through repositories.
- When the project marks one state holder as its reference implementation, new state holders follow it.
- Local path packages `core` `analytics_facade` own their dependencies: declare a dependency in the package that uses it, never in the root `pubspec.yaml`.
- Each feature lives in its own folder with a local `components/` folder; a component moves to a shared folder only once a second feature reuses it.
- Screens stay dumb: no business rules, calculations or decisions in widgets. Logic lives in the state holder.
- Async UI handles loading, empty, error (with retry where it applies) and success as distinct states, never as separate boolean flags.

## Flutter and Dart

- No `BuildContext` across an async gap: return early when the widget is no longer mounted, or capture what you need before the `await`.
- Dispose everything you create: controllers, focus nodes, stream subscriptions, timers, tickers, animation controllers, state holders created by hand.
- Use `const` constructors wherever possible; never rebuild a whole subtree for a local change.
- No `dynamic` in new code. No `!` unless the value is provably non-null in that scope.
- No `print(` in app code: use the project logger.
- Split a `build` method longer than 100 lines into feature components.
- No magic numbers; use named constants.
- Measure before optimizing (profile mode, DevTools); an unmeasured optimization is a guess. When refactoring, apply the `flutter-performance` skill first.
- Network images in lists and grids bound their decode size (`cacheWidth` / `cacheHeight`, `memCacheWidth`, or `ResizeImage`).

## Security

- HTTPS only; no cleartext endpoint.
- No API key, token or password in source. Sensitive values arrive through `--dart-define` (injected by `tool/build.sh`), never as literals.
- Tokens, refresh tokens, PINs and payment data go to secure storage or an encrypted store, never to plain storage or shared preferences. Non-sensitive cache may use plain storage.
- Never log tokens, auth responses or personal data: release logs are readable.
- Never send raw email, phone, full name or birth date as an analytics property; send an enum, boolean, count or year instead.
- A `badCertificateCallback` that returns true is a blocker; a debug-only use is guarded by `kDebugMode` and never reaches production.
- Deep-link parameters are untrusted: validate them before navigating or using them as ids, and re-check authorization for protected screens.
- The `mobile-security` skill has the full checklist.

## Testing

- Unit and state-holder tests live in `test/`, named `<subject>_test.dart` and kept flat unless the existing suite already uses subfolders. Integration journeys live in `integration_test/` and follow `flutter-add-integration-test`.
- Every feature ships data-layer tests (mapper, repository, model, state holder) in the same task, written first. A touched data-layer unit with no test is a high-severity finding.
- Add a test now, or report it as skipped in the summary; "later" is not an option.
- Scope runs to the diff: `flutter analyze <changed paths>` and `flutter test <covering test files>`. Run the whole repo only when the user asks or a release is cut.
- Say which scope ran. A scoped green run says nothing about the rest of the suite.
- Run all test files of a stage in one call, RED once and GREEN once per stage; aim for at most 6 test runs per task.
- Run `flutter test --reporter expanded` so test names are visible.
- There is no coverage threshold; never invent one.

## Debugging

- Read the relevant code before saying anything about a bug.
- In one pass, state what you found, where, and the fix.
- If the cause is unclear, or an error disappeared without a known reason, say so. Never guess, never claim it fixed.

## Output format

- No em dashes, smart quotes or decorative Unicode: plain hyphens and straight quotes. Natural-language characters are fine.
- Code output must be copy-paste safe.
- Never echo raw tool output or narrate; show only the code that changed.

- Prompts here are terse commands. Treat them as commands; answer short and blunt, with no explanation unless asked.

## Red flags

Stop and re-read these rules when you catch yourself: editing a file you have not read; adding an abstraction for one caller; writing a comment that explains what code does; shipping a changed public unit with no Dartdoc, or Dartdoc over the cap or narrating logic; finishing without the PR description file; opening a PR or merging without an explicit yes; committing or pushing against the Git rules; writing to the issue tracker unasked; claiming done without evidence.

## State: flutter_bloc

- Cross-feature blocs live in `lib/blocs/`; feature blocs and cubits live in `lib/features/<feature>/` under `bloc/`, `blocs/` or `cubit/`. Never add a fourth convention.
- A feature with two or more blocs gives each one its own `blocs/<name>/`; a single-bloc feature keeps `bloc/`. Reference: `lib/features/orders/blocs/`.
- Dynamic values travel on events (`OrderStarted(orderId: id)`); keep them in a private field when later events need them.
- The root screen wraps its `Scaffold` in `BlocProvider` or `MultiBlocProvider`.
- Never pass a Bloc or Cubit instance to a child widget, helper, sheet or dialog.
- Descendants use `context.read<T>()` in callbacks and `BlocBuilder` or `BlocSelector` in UI.
- A sheet or dialog that needs the parent bloc is wrapped in `BlocProvider.value(value: context.read<T>(), child: ...)`.
- Never call `context.read()` inside `build`; use `context.watch()` or `BlocBuilder` there. A violation is a review blocker.
- Every Bloc or Cubit class, and every event or state you add, carries Dartdoc.
- Bloc vs Cubit, state shape, rebuild scope and disposal: skill `bloc-patterns`.

## Dependency injection: get_it

- Register every new dependency in `lib/injection_container.dart`. A new dependency left unregistered is a high-severity review finding.
- Never add, remove or re-version `get_it` in `pubspec.yaml` without asking.

- Register every Bloc and Cubit with `getIt.registerFactory(() => OrderBloc())`; a local package registers in its own container.
- Screens obtain blocs with `getIt.get<OrderBloc>()`, never `OrderBloc(...)`.
- Bloc and Cubit constructors take no parameters: no repository, id, initial data, callback, or optional test seam.
- Repositories and use cases are fields: `final OrderRepository repository = getIt.get<OrderRepository>();`.

## Network: Dio

- Create or configure Dio (`Dio(`, `BaseOptions(`, interceptors) only under `lib/data/client/`. UI and blocs reach the network through repositories.

## Tests: manual fakes

- Tests use plain `flutter_test` with hand-written fakes (`implements`, or `extends` for a large interface). Never add mocktail, mockito or bloc_test.

## Local storage: Hive

- Boxes are read and written from repositories or `LocalDatabase`, never from widgets or entrypoints.
- Never rename a key already stored on user devices without a migration.

## Analytics: PostHog

- On every feature change, flag whether it adds, renames or removes a user action, screen or funnel step, then ask the user whether to track it.
- Never wire an event without that decision. Roles only list candidates; the final review flags wiring without a decision.
- The step 0 prompt names the candidate actions and screens. The summary and the PR description's `## Analytics` section record the decision: added, declined or not relevant.
- Read `docs/analytics.md` (events, funnels, properties) before touching analytics.
- Call `Analytics.instance` from the `analytics` package. Never import `package:posthog_flutter` in `lib/`.
- Event, property, screen and element names are constants in `analytics_constants.dart`. No string literals.
- Never rename an event or property in use; it breaks historical funnels.
- Fire from callbacks or state listeners, never inside `build()` or a builder.

## QA identifiers

- When slicing or redesigning a screen, tag every QA-relevant widget: anything a test taps, types into or asserts on (buttons, inputs, toggles, checkboxes, list items, tabs, nav actions, dialogs, sheets, key status or amount text). Skip decorative and layout-only widgets.
- Wrap with `Semantics(key: ValueKey(id), identifier: id)`, the same string in both. `identifier` reaches native tools; `key` reaches `find.byKey`.
- Names are snake_case, screen-prefixed, semantic and stable across builds: `order_list_checkout_button`.
- No hash codes, indexes or timestamps. Repeated items key off a stable domain id: `order_item_${order.id}`.
- Never remove or rename an identifier without checking the automation suites. A missing one is high severity until proven unreferenced.
- The step 0 prompt lists the identifiers to add. The PR description's `## QA Identifiers` section lists those added, changed or removed, or "None".

<!-- dashcrew:project:begin -->
<!-- dashcrew:project:end -->
