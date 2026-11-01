---
target: agents_md
order: 300
rules: [bloc.placement, bloc.folder-per-bloc, bloc.dynamic-params-via-events, bloc.scaffold-wrapped-in-provider, bloc.no-passing-instances, bloc.access-via-context, bloc.sheet-provider-value, bloc.read-vs-watch, doc.required.bloc-events-states]
---
## State: flutter_bloc

- Cross-feature blocs live in `{{pack.state.shared_dir}}`; feature blocs and cubits live in `{{pack.state.feature_dir}}` under `bloc/`, `blocs/` or `cubit/`. Never add a fourth convention.
- A feature with two or more blocs gives each one its own `blocs/<name>/`; a single-bloc feature keeps `bloc/`. Reference: `{{pack.state.reference_folder}}`.
- Dynamic values travel on events (`OrderStarted(orderId: id)`); keep them in a private field when later events need them.
- The root screen wraps its `Scaffold` in `BlocProvider` or `MultiBlocProvider`.
- Never pass a Bloc or Cubit instance to a child widget, helper, sheet or dialog.
- Descendants use `context.read<T>()` in callbacks and `BlocBuilder` or `BlocSelector` in UI.
- A sheet or dialog that needs the parent bloc is wrapped in `BlocProvider.value(value: context.read<T>(), child: ...)`.
- Never call `context.read()` inside `build`; use `context.watch()` or `BlocBuilder` there. A violation is a review blocker.
- Every Bloc or Cubit class, and every event or state you add, carries Dartdoc.
- Bloc vs Cubit, state shape, rebuild scope and disposal: skill `bloc-patterns`.
