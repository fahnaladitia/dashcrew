---
name: build-fix
description: "Diagnose and fix Flutter, code generation, dependency, Gradle and CocoaPods build errors one at a time, with real verification."
---

Arguments: `[error output or failing command]`

# build-fix

The main session runs it. Read the error and the related code before forming any theory; never guess.
This is not a chain step: use it when a build error blocks any step, then resume where you were.

Never start with a clean. It destroys evidence and costs minutes. Read the error first.

## Diagnose in this order

### 1. Analyze

```bash
flutter analyze 2>&1 | head -40
```

If the error shows up here, it is a code error, not a build problem. Fix it here and stop.

### 2. Generated code

Symptom: a missing `.g.dart` file or a missing generated symbol.

```bash
dart run build_runner build --delete-conflicting-outputs
```

A change to a persisted type's ids or field order breaks data already stored on user devices; raise it to the user, never change it silently.

### 3. Dependency resolution

```bash
flutter pub get 2>&1 | tail -20
```

### 4. Android / Gradle

```bash
cd android && ./gradlew assembleDebug --stacktrace 2>&1 | tail -40
```

Common causes: minSdk too low for a new dependency, duplicate classes from transitive dependencies, Kotlin or AGP version mismatch, manifest merge conflicts.

### 5. iOS / CocoaPods

```bash
cd ios && pod install 2>&1 | tail -30
```

Common causes: `Podfile.lock` out of sync after a dependency change, deployment target below a pod's minimum, simulator architecture issues on Apple Silicon.

### 6. Last: clean

Only after steps 1 to 5, and only when the error still makes no sense, run `flutter clean` followed by `flutter pub get`.

## Rules

- One error, one fix, one rebuild. Never stack fixes before rebuilding.
- If an error disappears and you do not know why, say so. Do not claim it is fixed.
- Touch no line outside the cause of the error.

## Output

```
BUILD FIX

Error:   <original message, trimmed>
Cause:   <root cause, at file:line>
Fix:     <what changed>
Verify:  <command run, with its real output>
```

Paste the command output. A claim without output is not verification.

## Dependency conflicts

- Never bump a version in `pubspec.yaml` without the user's approval.
- Read the conflicting constraints before proposing anything.
- Version-solving failures usually start in local path packages and vendored packages, not in pub.dev packages. Check those first.
- For a `pub get` version conflict, apply `dart-resolve-package-conflicts`.

## Real builds

- When the project defines flavors, run and build each flavor with its own entrypoint (`flutter run --flavor <flavor> -t <entrypoint>`); the flavor and entrypoint pairs are listed in AGENTS.md.
- When flavors exist, build Android with the flavor's Gradle task (`assemble<Flavor>Debug`), not the plain `assembleDebug`.
