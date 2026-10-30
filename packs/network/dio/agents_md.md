---
target: agents_md
order: 320
rules: [network.dio-only-in-client]
---
## Network: Dio

- Create or configure Dio (`Dio(`, `BaseOptions(`, interceptors) only under `{{pack.network.client_dir}}`.{{#pack.network.tool_exception_dir}} `{{.}}` (a developer tool) is the only exception.{{/pack.network.tool_exception_dir}} UI and blocs reach the network through repositories.
