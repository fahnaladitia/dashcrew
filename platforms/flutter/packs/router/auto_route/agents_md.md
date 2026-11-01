---
target: agents_md
order: 315
rules: [router.auto-route-page-annotation, router.auto-route-one-router, router.auto-route-navigation, router.auto-route-path-params, router.auto-route-guards]
---
## Navigation: auto_route

- Every routable page is annotated `@RoutePage()` and listed in the `@AutoRouterConfig()` router in `{{pack.router.router_file}}`; rerun `build_runner` after a change and never edit the generated file.
- The router is created once, in a field, and its `config()` goes to `MaterialApp.router`; never inside `build`.
- Navigate with generated route objects through `context.router`: `push` adds, `replace` swaps the top page, `navigate` returns to an existing route. Type results (`push<bool>`) and return them with `maybePop(value)`.
- Path segments bind with `@PathParam('id')`; a wildcard route is listed last.
- Gates are `AutoRouteGuard`s that call `resolver.next` exactly once.
