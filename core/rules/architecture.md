---
target: agents_md
order: 70
rules: [mapper.data-flow, mapper.never-skip, mapper.no-dto-in-ui, mapper.defensive-parsing, arch.data-layer-folders, arch.ui-through-repository, bloc.reference-implementation, storage.dependency-location, ui.feature-folder, ui.dumb-screens, ui.exhaustive-async-states, ui.no-flag-soup]
---
## Architecture

- Data flows request -> client -> response DTO -> mapper -> model -> repository -> state holder -> UI. Never skip the mapper, and never let a response DTO leak past the repository.
- Data-layer code follows the project's existing folder layout, one folder per layer; never invent a new one.
- Mappers parse defensively with fallbacks: a default for a missing or mistyped number, an empty list for a missing list.
- UI and state holders reach the network only through repositories.
- When the project marks one state holder as its reference implementation, new state holders follow it.
- Local path packages{{#project.local_packages}} `{{.}}`{{/project.local_packages}} own their dependencies: declare a dependency in the package that uses it, never in the root dependency manifest.
- Each feature lives in its own folder with a local `components/` folder; a component moves to a shared folder only once a second feature reuses it.
- Screens stay dumb: no business rules, calculations or decisions in UI components. Logic lives in the state holder.
- Async UI handles loading, empty, error (with retry where it applies) and success as distinct states, never as separate boolean flags.
