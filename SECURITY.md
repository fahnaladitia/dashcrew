# Security policy

## Supported versions

Only the latest minor release gets security fixes, as a new patch release. Older minors are not patched; upgrade to the latest one.

| Version | Supported |
|---|---|
| latest minor (`0.y.z` with the highest `y`) | yes |
| older minors | no |
| `main` before the first tagged release | yes, fixed on `main` |

## Reporting a vulnerability

Report it privately through GitHub's private vulnerability reporting:

https://github.com/fahnaladitia/dashcrew/security/advisories/new

Do not open a public issue, pull request or discussion for a vulnerability. Include the dashcrew version (`dashcrew --version`), the OS, the agent the hook ran for, and the smallest input that shows the problem (for the guard: the hook JSON and the policy).

## Response

- Acknowledgement within 7 days.
- A first assessment (accepted or not, and the planned fix) within 14 days.
- A fix released within 90 days of the report for an accepted issue, sooner for a severe one. The advisory is published with the release, crediting the reporter unless they ask not to be named.

The project has one maintainer; these are targets, not guarantees.

## Scope

In scope: the `dashcrew` binary and its subcommands, the guard shim `.ai/bin/guard` and the files `sync` generates, the release workflow and the published release artifacts.

Notes:

- **The guard prevents mistakes, not a determined attacker.** It stops an AI agent from running listed commands (push, history rewrites, formatters, and the others in the README's [Guard](README.md#guard) section) by accident or by following bad instructions. It is not a sandbox. An agent or a person who sets out to get around it can, for example through the documented blind spots: a first word that is a `$` expansion, `source` or `.`, `fish -c`, `stdbuf`, `doas`, or nesting deeper than 8 levels. These are listed under "Hidden commands" in the README and are not vulnerabilities by themselves; a way around the guard that the README does not list is worth reporting.
- **Fail open on malformed input.** A hook payload the guard cannot read, and a missing or broken policy file, let the call through by design (see "Malformed payloads" in the README). `doctor` reports both. A payload that the agent sends in its documented format and that the guard misreads is in scope.
- **Fail closed on a missing binary.** The shim refuses every tool call when `dashcrew` is missing or too old. A way to make the shim run something other than the `dashcrew` on `PATH` is in scope.
- Out of scope: vulnerabilities in the AI agents themselves, in git, in the shell, or in a project's own packs and platforms; findings that need write access to the project's `.ai/` folder or to `PATH` beforehand.
