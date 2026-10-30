---
name: guardrails
description: "What the generated guard hooks block or warn about, and why a command was refused."
when_to_use: "When a guard blocks a command or prints a post-edit warning."
---

# Guardrails

Guards back the Git and code rules in AGENTS.md mechanically. A block is the rule working: do not route around it, follow the rule it names.

## Command guard

- It matches a blocked command only in command position: at the start of a line or after a shell separator (`;`, `&&`, `||`, `|`). It sees through `sudo` and command-proxy prefixes.
- Searching for text such as a blocked command name (for example with `grep`) is not blocked.
- Text inside a heredoc that starts a line with a blocked command can still be refused. Write such content with the file-writing tool instead of the shell.

## Post-edit check

- After a Dart file is edited, a cheap pattern check prints warnings. It never blocks and never runs the analyzer.
- Generated files are skipped, and output stops after 10 findings.

## Boundaries

- Checks on the app's `lib/` boundaries apply to the main app only. Local path packages `core` `analytics_facade` have their own `lib/` and are not held to the app's layer rules.
