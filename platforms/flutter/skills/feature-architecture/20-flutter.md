---
target: skill/feature-architecture
order: 20
rules: [arch.flutter-widget-split, arch.flutter-dispose-list]
---
## In Flutter

- Components are widget classes that can be `const`, never `_buildXxx()` methods; the render method is `build`.
- Rebuild boundaries use the state pack's selective tools (selectors, rebuild conditions) instead of rebuilding the screen.
- The dispose list names every controller, focus node, stream subscription, timer, ticker and state holder created by hand.
