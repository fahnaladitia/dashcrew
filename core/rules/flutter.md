---
target: agents_md
order: 71
rules: [flutter.no-context-across-async, flutter.dispose-everything, flutter.const-constructors, flutter.no-dynamic, flutter.no-unproven-bang, flutter.no-print, flutter.build-max-100-lines, flutter.named-constants, perf.measure-first, perf.apply-on-refactor, perf.image-cache-bounds]
---
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
