---
name: build-fix
description: "Diagnose and fix build, code generation and dependency errors one at a time, with real verification."
argument-hint: "[error output or failing command]"
---

# build-fix

The main session runs it. Read the error and the related code before forming any theory; never guess.
This is not a chain step: use it when a build error blocks any step, then resume where you were.

Never start with a clean. It destroys evidence and costs minutes. Read the error first.

## Diagnose in this order

### 1. Analyze

```bash
./scripts/lint.sh 2>&1 | head -40
```

If the error shows up here, it is a code error, not a build problem. Fix it here and stop.

### 2. Generated code

Symptom: a missing generated file or a missing generated symbol. Rerun the project's code generation step as its build scripts define it.

A change to a persisted type's ids or field order breaks data already stored on user devices; raise it to the user, never change it silently.

### 3. Dependency resolution

```bash
./scripts/deps.sh 2>&1 | tail -20
```

### 4. The failing build

Rerun the failing build command with its most verbose output, and read the first error, not the last.

### 5. Last: clean

Only after steps 1 to 4, and only when the error still makes no sense, clean the build output, then run `./scripts/deps.sh` again.

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

- Never bump a version in the dependency manifest without the user's approval.
- Read the conflicting constraints before proposing anything.
- Version-solving failures usually start in local path packages and vendored packages, not in registry packages. Check those first.
