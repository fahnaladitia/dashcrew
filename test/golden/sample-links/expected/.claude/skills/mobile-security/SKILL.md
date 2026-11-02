---
name: mobile-security
description: "Security checklist for a Flutter mobile app - credentials, storage, transport, personal data, platform surface, scanners and verdict."
when_to_use: "When the scope touches auth or tokens, storage, the network client, deep links, WebViews, analytics properties, permissions, platform config or logs of user data."
---

# Mobile security

The binary is in the attacker's hands and can be unpacked. The always-on rules (HTTPS, secrets, secure storage, logs, analytics properties, TLS validation, deep links) are in AGENTS.md. This checklist adds the rest.

## Platform files

- Never put keys in `AndroidManifest.xml`, `Info.plist` or committed `.json` files.
- `usesCleartextTraffic` and `NSAllowsArbitraryLoads` are off in production.
- Flag `allowBackup`, exported components without a permission, and `debuggable`; `debuggable` is off in production.
- Screens that show payment or personal data should set `FLAG_SECURE` on Android, and consider blocking task-switcher snapshots on iOS.

## WebView

- Unrestricted JavaScript is allowed only for an allowlist of hosts.
- Never pass a token in a WebView URL query string.

## Scanners

- Run scanner scripts with `python3 -I` and write their JSON output to a scratch directory, never the repo. Afterwards check `git status` and move any stray output out of the repo root.
- A scanner hit is a candidate. Read the code to confirm it; never report an unverified finding.

## Review checklist

1. Credentials in the binary: keys, tokens and passwords in source or platform folders; secrets come from `--dart-define`, never literals.
2. Local storage: tokens, refresh tokens, PINs and payment data in secure or encrypted storage only.
3. Transport: cleartext HTTP, a `badCertificateCallback` that returns true outside `kDebugMode`, platform flags allowing cleartext.
4. Personal data: no tokens, auth responses or personal data in logs; no raw email, phone, full name or birth date in analytics properties.
5. Platform surface: deep-link parameters validated before navigation or use as ids; WebViews restricted to allowed hosts; manifest flags above.
6. OWASP Mobile Top 10 references for any category the items above do not cover.

## Verdict

- Report in the `code-review` format. A CRITICAL security issue is a BLOCKER; any BLOCKER or HIGH means BLOCK.
- Every finding shows `file:line`, the fix, and a one-line impact.
