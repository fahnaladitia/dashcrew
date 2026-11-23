---
target: agents_md
order: 320
rules: [network.chopper-generated-services, network.chopper-one-client, network.chopper-converter, network.chopper-errors-in-repository]
---
## Network: Chopper

- Services are `@ChopperApi` abstract classes with a static `create` returning `_$ClassName`, in `{{pack.network.client_dir}}`. Rerun `build_runner` after a change; never edit `*.chopper.dart`.
- One configured `ChopperClient` (base URL, interceptors, converter) lists every service.
- Its converter decodes bodies into models; never decode JSON by hand.
- A non-2xx answer is a `Response` with `isSuccessful` false, not an exception. Repositories map it, and transport errors, to app errors.
