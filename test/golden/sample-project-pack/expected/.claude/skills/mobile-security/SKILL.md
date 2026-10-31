---
name: mobile-security
description: "Security checklist for a Flutter mobile app - secrets, storage, transport, platform config, scanners and verdict."
when_to_use: "When a change touches auth, tokens, storage, networking, deep links, WebViews or platform config, and in a security review."
---

# Mobile security

The always-on rules (HTTPS, secrets, secure storage, logs, analytics properties, TLS validation, deep links) are in AGENTS.md. This checklist adds the rest.

## Platform files

- Never put keys in `AndroidManifest.xml`, `Info.plist` or committed `.json` files.
- `usesCleartextTraffic` and `NSAllowsArbitraryLoads` are off in production.
- Flag `allowBackup`, exported components without a permission, and `debuggable`; `debuggable` is off in production.
- Screens that show payment or personal data should set `FLAG_SECURE` on Android.

## WebView

- Unrestricted JavaScript is allowed only for an allowlist of hosts.
- Never pass a token in a WebView URL query string.

## Scanners

- Run scanner scripts with `python3 -I` and write their JSON output to a scratch directory, never the repo. Afterwards check `git status` and move any stray output out of the repo root.
- A scanner hit is a candidate. Read the code to confirm it; never report an unverified finding.

## Verdict

- Severity is CRITICAL, HIGH or MEDIUM. Any CRITICAL or HIGH means BLOCK.
- Every finding shows `file:line`, the fix, and the impact.
