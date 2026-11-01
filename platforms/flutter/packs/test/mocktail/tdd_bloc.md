---
target: workflow/tdd
order: 305
rules: [tdd.bloc-test]
when: { stack.state.use: bloc }
---
### blocTest

Bloc and Cubit tests use `blocTest` from `bloc_test`: `build` returns the bloc, `act` drives it, `expect` lists every state in order (extra states fail). `seed` sets a starting state; `wait` covers debounced events. Widget tests stub a bloc with `MockBloc` or `MockCubit` and `whenListen`.
