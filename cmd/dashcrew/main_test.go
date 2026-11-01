package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dashcrew(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, env)
	return code, stdout.String(), stderr.String()
}

func TestSyncWritesTheToolkitThenReportsUpToDate(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")

	code, stdout, stderr := dashcrew(t, nil, "sync", "--no-compile-guard", "--project", project)
	if code != 0 || !strings.HasPrefix(stdout, "wrote   AGENTS.md\nwrote   .ai/guard.json\nwrote   .ai/bin/guard\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if !strings.HasSuffix(stdout, "wrote   .claude/settings.json\nwrote   .mcp.json\n") {
		t.Errorf("stdout %q", stdout)
	}

	if code, stdout, _ := dashcrew(t, nil, "sync", "-p", project); code != 0 || stdout != "up to date\n" {
		t.Errorf("second sync: exit %d, stdout %q", code, stdout)
	}
}

func TestSyncPrintsRemovedFilesWarningsAndExitsOneWhenRefused(t *testing.T) {
	project := t.TempDir()
	profile := filepath.Join(project, ".ai", "project.yaml")
	writeFile(t, profile, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\npolicy: { plan_gate: true }\n")
	if code, _, stderr := dashcrew(t, nil, "sync", "--project", project); code != 0 ||
		!strings.Contains(stderr, "warning: plan_gate was removed in P4; delete it from .ai/project.yaml\n") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	writeFile(t, profile, "toolkit: 1\nagents: [claude-code]\n")

	code, stdout, _ := dashcrew(t, nil, "sync", "--project", project)
	if code != 0 || !strings.Contains(stdout, "removed .claude/skills/bloc-patterns/SKILL.md\n") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}

	writeFile(t, filepath.Join(project, ".claude", "agents", "reviewer.md"), "mine\n")
	code, _, stderr := dashcrew(t, nil, "sync", "--project", project)
	if code != 1 || !strings.HasPrefix(stderr, "Files changed outside dashcrew (rerun with --force to overwrite):\n  .claude/agents/reviewer.md") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := dashcrew(t, nil, "sync", "--project", project, "--force"); code != 0 {
		t.Errorf("--force exit %d", code)
	}
}

func TestSyncReportsAProfileError(t *testing.T) {
	project := t.TempDir()

	if code, _, stderr := dashcrew(t, nil, "sync", "--project", project); code != 1 || !strings.Contains(stderr, "No .ai/project.yaml") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestDoctorExitsOneOnAProjectThatWasNeverSynced(t *testing.T) {
	project := t.TempDir()

	code, stdout, _ := dashcrew(t, nil, "doctor", "--project", project)

	if code != 1 || !strings.Contains(stdout, "fail  profile         .ai/project.yaml missing; run dashcrew init\n") ||
		!strings.Contains(stdout, "fail  lock            .ai/lock.json missing; run dashcrew sync\n") ||
		!strings.Contains(stdout, "skip  drift           needs the lock\n") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestCheckFailsAPrBodyThatMissesASectionAndAnnotatesItInGithubActions(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	if code, _, stderr := dashcrew(t, nil, "sync", "--no-compile-guard", "--project", project); code != 0 {
		t.Fatalf("sync: %s", stderr)
	}
	body := filepath.Join(project, "body.md")
	writeFile(t, body, "## Summary\nx\n")

	code, stdout, _ := dashcrew(t, map[string]string{"GITHUB_ACTIONS": "true"}, "check", "--project", project, "--pr-body", body)

	if code != 1 || !strings.Contains(stdout, "fail  pr-sections") ||
		!strings.Contains(stdout, "::error title=dashcrew pr-sections::missing: ## Changes") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}

	code, stdout, _ = dashcrew(t, nil, "check", "--project", project, "--pr-body", body)
	if code != 1 || strings.Contains(stdout, "::error") {
		t.Errorf("outside GitHub Actions: exit %d, stdout %q", code, stdout)
	}
}

func TestCheckPassesACompletePrBody(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	if code, _, stderr := dashcrew(t, nil, "sync", "--project", project); code != 0 {
		t.Fatalf("sync: %s", stderr)
	}
	body := filepath.Join(project, "body.md")
	writeFile(t, body, "## Summary\n## Changes\n## Affected files\n## Testing\n## QA Identifiers\n## Analytics\n## Notes / not done\n")

	code, stdout, _ := dashcrew(t, map[string]string{"GITHUB_ACTIONS": "true"}, "check", "-p", project, "--pr-body", body)

	if code != 0 || !strings.Contains(stdout, "ok    pr-sections     5 required sections present\n") || strings.Contains(stdout, "::error") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestUsageErrors(t *testing.T) {
	if code, stdout, _ := dashcrew(t, nil); code != 0 || !strings.Contains(stdout, "Usage: dashcrew") {
		t.Errorf("no command: exit %d", code)
	}
	if code, _, stderr := dashcrew(t, nil, "gate", "status"); code != 64 || !strings.Contains(stderr, `Could not find a command named "gate"`) {
		t.Errorf("gate: exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := dashcrew(t, nil, "doctor", "--nope"); code != 64 {
		t.Errorf("unknown flag: exit %d", code)
	}
	if code, _, stderr := dashcrew(t, nil, "check", "--pr-body", filepath.Join(t.TempDir(), "missing.md")); code != 64 ||
		!strings.Contains(stderr, "not found") {
		t.Errorf("missing body: exit %d, stderr %q", code, stderr)
	}
}

func TestGuardBlocksThroughTheSyncedPolicyAndRefusesInstall(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	if code, _, stderr := dashcrew(t, nil, "sync", "-p", project); code != 0 {
		t.Fatalf("sync: %s", stderr)
	}

	stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push"},"cwd":` + strconv.Quote(project) + `}`)
	t.Cleanup(func() { stdin = os.Stdin })
	code, stdout, stderr := dashcrew(t, nil, "guard")
	if code != 2 || stdout != "" || stderr != "[dashcrew guard] git push is reserved for the user. Ask them to run it.\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	if code, _, stderr := dashcrew(t, nil, "guard", "install", "--expect", "x"); code != 64 || !strings.Contains(stderr, "Dart engine") {
		t.Errorf("install: exit %d, stderr %q", code, stderr)
	}
}
