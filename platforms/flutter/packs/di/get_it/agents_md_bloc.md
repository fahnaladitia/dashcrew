---
target: agents_md
order: 311
rules: [bloc.register-factory, bloc.screen-gets-from-locator, bloc.no-constructor-params, bloc.repository-via-field]
when: { stack.state.use: bloc }
---
- Register every Bloc and Cubit with `{{pack.di.locator}}.registerFactory(() => OrderBloc())`; a local package registers in its own container.
- Screens obtain blocs with `{{pack.di.locator}}.get<OrderBloc>()`, never `OrderBloc(...)`.
- Bloc and Cubit constructors take no parameters: no repository, id, initial data, callback, or optional test seam.
- Repositories and use cases are fields: `final OrderRepository repository = {{pack.di.locator}}.get<OrderRepository>();`.
