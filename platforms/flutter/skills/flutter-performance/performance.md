---
target: skill/flutter-performance
rules: [perf.reuse-image-cache-package, perf.no-widget-builder-methods, perf.list-builders, perf.stable-item-keys, perf.avoid-shrinkwrap, perf.nested-scroll-physics, perf.no-intrinsic-in-lists, perf.mediaquery-size-of, perf.no-heavy-build, perf.no-future-in-build, perf.compute-heavy-work]
---
# Flutter performance

Measure first: run in profile mode, use DevTools and the rebuild debug flag. An optimization without a measurement is a guess.

## Rebuilds

- Build sub-widgets as separate widget classes that can be `const`, not as `_buildXxx()` methods called from `build`.
- Scope rebuilds to the part of the state that changed (selectors, rebuild conditions) instead of rebuilding the whole screen.
- Use `MediaQuery.sizeOf(context)` instead of `MediaQuery.of(context)` when only the size is needed.

## Work in build

- No parsing, sorting, filtering, regex or calculation in `build`; do it in the state holder and store the result in state.
- Never create a `FutureBuilder` future inside `build`; create it in `initState` or the state holder.
- Run heavy operations through `compute` so they do not block the UI thread.

## Lists

- Use `ListView.builder` / `GridView.builder` for long or unbounded lists.
- Reorderable items get stable keys from domain ids (`ValueKey(order.id)`), never from indexes.
- Avoid `shrinkWrap: true` on long lists.
- Give nested scroll views the right `physics` so they are not measured repeatedly.
- No `IntrinsicHeight` / `IntrinsicWidth` inside scrolling lists.

## Images

- Bound decode size for network images in lists and grids (`cacheWidth` / `cacheHeight`, `memCacheWidth`, `ResizeImage`).
- Use the image caching package the project already depends on; never build a custom cache.
