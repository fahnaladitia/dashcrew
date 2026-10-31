---
target: agents_md
order: 320
rules: [network.retrofit-generated-clients, network.retrofit-one-dio, network.retrofit-from-json, network.retrofit-errors-in-repository]
---
## Network: Retrofit

- API classes are abstract classes annotated `@RestApi()` with a factory redirecting to the generated `_ClassName`, kept in `{{pack.network.client_dir}}` with `part '<file>.g.dart';`. Rerun `build_runner` after a change; never edit `*.g.dart`.
- One configured `Dio` (base URL, interceptors) is passed to every API class.
- Every response model has `fromJson` (json_serializable); an enum used in a path or query overrides `toString` with its wire value.
- Failures arrive as `DioException`; repositories map them to app errors, and the UI never sees them.
