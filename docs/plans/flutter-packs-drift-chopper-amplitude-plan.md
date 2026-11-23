# Flutter packs: drift, chopper, amplitude

Tier: standard (3+ files in catalog, initproject and docs; generated output changes only for a profile that selects a new option, plus the catalog digest in the lock goldens). Not large: each pack joins an existing family, no new family or layout.

Issues: #56 (drift), #57 (chopper), #58 (amplitude). One commit per pack, so each can be reviewed and reverted alone.

## Goal

`init` proposes `storage: drift`, `network: chopper` and `analytics: amplitude` as packs instead of `use: other`, and each gives agents the library's own rules, one line each, in the format of the sibling pack.

## Per pack

### drift (#56): `platforms/flutter/packs/storage/drift/`

Modelled on `storage/sqflite`.

- `pack.yaml`: description; params `database_class: AppDatabase`.
- `agents_md.md` (`order: 340`, heading `## Local storage: drift`), rules from drift's documentation:
  - One `{{pack.storage.database_class}}` (`@DriftDatabase`), opened once and reused; repositories reach it through that class or its DAOs, never widgets.
  - Tables and queries are Dart classes or `.drift` files; rerun `build_runner` after a change and never edit `*.g.dart`.
  - A schema change bumps `schemaVersion` and adds a step to `onUpgrade` (`stepByStep` from `drift_dev make-migrations` when the project uses it). Never edit a migration that already shipped.
  - Raw SQL (`customSelect`, `customStatement`) takes values through `variables`, never string interpolation.
  - Writes that must succeed together run in one `transaction`; await every query inside it.
  - Reactive reads use `watch()`; repositories map rows to domain models before the stream leaves the data layer.
- `tdd.md` (`order: 340`, `### Database tests`): open `{{pack.storage.database_class}}(NativeDatabase.memory())` per test and close it in `tearDown`; with `make-migrations`, test each step with the generated schema verifier.
- Rule ids `storage.drift-*` plus the shared `storage.access-via-repository`, as sqflite does.

### chopper (#57): `platforms/flutter/packs/network/chopper/`

Modelled on `network/retrofit`.

- `pack.yaml`: description; params `client_dir: lib/data/client/`.
- `agents_md.md` (`order: 320`, heading `## Network: Chopper`):
  - Services are abstract `ChopperService` classes annotated `@ChopperApi`, with a static `create` returning the generated `_$Name`, kept in `{{pack.network.client_dir}}` with `part '<file>.chopper.dart';`. Rerun `build_runner` after a change; never edit `*.chopper.dart`.
  - One configured `ChopperClient` (base URL, interceptors, converter, error converter) lists every service; services are read from it with `getService`.
  - Models are decoded by the client's converter, never inside a service or repository by hand.
  - A non-2xx answer is a `Response` with `isSuccessful` false, not an exception; repositories check it and map it, and thrown transport errors, to app errors. The UI never sees a `Response`.
- `tdd.md` (`order: 320`, `### Network tests`): pass `MockClient` from `package:http/testing.dart` as the `ChopperClient`'s `client` and test the repository against canned responses, including a non-2xx one.

### amplitude (#58): `platforms/flutter/packs/analytics/amplitude/`

A copy of `analytics/mixpanel` (`agents_md.md`, `plan.md`, `pr_template.md`, `summary.md`, `tdd.md`), with:

- `pack.yaml`: mixpanel's params, `vendor_import: package:amplitude_flutter`, and `server_zone: null` in place of `server_url`.
- The last `agents_md.md` line: one `Amplitude(Configuration(apiKey: ...{{#pack.analytics.server_zone}}, serverZone: ServerZone.{{.}}{{/pack.analytics.server_zone}}))` built at startup and awaited through `isBuilt` before the first event. Events are batched; call `flush()` only when an event must leave before the app closes.
- Heading `## Analytics: Amplitude`.

## AGENTS.md budget

`TestLargestOptionOfEveryFamilyStaysUnderTheLimit` renders the largest option of every family together and allows 23,788 bytes; the catalog left 5. So each new pack's `agents_md.md` is no larger, rendered, than the largest option already in its family (`retrofit`, `shared_preferences`, `mixpanel`). As built, the largest set is unchanged (23,783 bytes). That cut two lines drafted above: drift's `watch()` line, and chopper's error converter and `getService` mentions (`getService` stays in the test example).

## Shared changes

- `internal/initproject/detect.go`: `drift` and `chopper` from `otherRule` to `rule`; new `"amplitude_flutter": {rule("analytics", "amplitude")}`. Rows in `detect_test.go`.
- `docs/profile.md`: add `drift`, `chopper`, `amplitude` to the `storage`, `network`, `analytics` rows.
- `docs/catalog-format.md` lists families, not options: no change.

## Tests first

- `internal/initproject/detect_test.go`: `drift` -> `storage: drift`, `chopper` -> `network: chopper`, `amplitude_flutter` -> `analytics: amplitude`.
- `internal/render/catalog_render_test.go`: `TestPackHeadings` rows for the three headings; `TestPrTemplate` loops over `amplitude` too.
- `internal/guard/policy_test.go`: a profile with `analytics: amplitude` compiles `ForbiddenImports == ["package:amplitude_flutter"]`, and that policy blocks a `Write` of `import 'package:amplitude_flutter/amplitude.dart';` to `lib/` (allowed in `test/` and a local package), as the posthog case does.

## Goldens

`DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard`. Expected: only the catalog digest in lock goldens changes; no golden profile selects a new option. Any other changed file is a bug to look at before merging. `internal/guard/testdata/corpus_outcomes.txt` must not change.

## Verify

`gofmt -l .`, `go vet ./...`, `go test ./...`. Then `dashcrew sync` in a scratch Flutter project with each new option, and read the rendered AGENTS.md section.

## Not doing

- A `drift` skill (hive has one); add it later if the rules prove too short.
- Detecting `chopper_generator` or `drift_dev` alone; the runtime package is the signal, as for the other packs.
