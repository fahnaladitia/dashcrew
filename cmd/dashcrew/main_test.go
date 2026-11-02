package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
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

	code, stdout, stderr := dashcrew(t, nil, "sync", "--project", project)
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
	if code, _, stderr := dashcrew(t, nil, "sync", "--project", project); code != 0 {
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

	if code, _, stderr := dashcrew(t, nil, "guard", "install", "--expect", "x"); code != 64 ||
		stderr != "dashcrew guard install was removed: hooks run dashcrew guard from PATH. Run dashcrew sync to update .ai/bin/guard.\n" {
		t.Errorf("install: exit %d, stderr %q", code, stderr)
	}
}

func TestVersionPrintsTheLineTheShimReads(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--version", "sync"}, {"sync", "--version"}} {
		if code, stdout, stderr := dashcrew(t, nil, args...); code != 0 || stdout != "dashcrew 0.0.1\n" || stderr != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	if code, _, _ := dashcrew(t, nil, "--version=1"); code != 64 {
		t.Errorf("--version=1: exit %d", code)
	}
}

func TestCompileGuardIsADeprecatedNoOpWithANotice(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, flag := range []string{"--compile-guard", "--no-compile-guard"} {
		if code, _, stderr := dashcrew(t, nil, "sync", flag, "-p", project); code != 0 || stderr != compileGuardNotice+"\n" {
			t.Errorf("%s: exit %d, stderr %q", flag, code, stderr)
		}
	}
	if code, _, stderr := dashcrew(t, nil, "sync", "-p", project); code != 0 || stderr != "" {
		t.Errorf("without the flag: exit %d, stderr %q", code, stderr)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("sync wrote into HOME: %v", entries)
	}
}

func TestHelpListsUsage(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		if code, stdout, stderr := dashcrew(t, nil, args...); code != 0 || !strings.Contains(stdout, "Usage: dashcrew") || stderr != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestInitYesWritesTheProfileAndASecondRunRefuses(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "pubspec.yaml"), "name: app\ndependencies:\n  dio: ^5.0.0\n")

	code, stdout, stderr := dashcrew(t, nil, "init", "--project", project, "--yes")
	if code != 0 || !strings.Contains(stdout, "next: dashcrew sync") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	profile, err := os.ReadFile(filepath.Join(project, ".ai", "project.yaml"))
	if err != nil || !strings.Contains(string(profile), "  network: dio\n") {
		t.Errorf("profile %q, %v", profile, err)
	}

	if code, _, stderr := dashcrew(t, nil, "init", "--project", project, "--yes"); code != 1 || !strings.Contains(stderr, "--force") {
		t.Errorf("second run: exit %d, stderr %q", code, stderr)
	}
}

func TestWorktreeCreateListAndRemove(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", app},
		{"-C", app, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	worktree := func(args ...string) (int, string, string) {
		return dashcrew(t, nil, append(append([]string{"worktree"}, args...), "--project", app)...)
	}

	if code, stdout, stderr := worktree("create", "feat/a", "main", "--no-pub"); code != 0 || !strings.Contains(stdout, "upstream: none") {
		t.Fatalf("create: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, stdout, _ := worktree("list"); !strings.Contains(stdout, "feat-a  feat/a  base main  upstream -  dirty 0") {
		t.Errorf("list: %q", stdout)
	}
	if code, _, stderr := worktree("remove", "nope"); code != 1 || !strings.Contains(stderr, "No worktree named nope") {
		t.Errorf("remove nope: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := worktree("remove", "feat-a"); code != 0 {
		t.Errorf("remove: exit %d, stderr %q", code, stderr)
	}
}

// TestGuardInstallIsRemovedAndMalformedPayloadsReachDoctor pins `guard install` (removed, exit 64), a guard hook
// without a readable policy (allowed silently), and the malformed-payload log that sync stamps and doctor reports.
func TestGuardInstallIsRemovedAndMalformedPayloadsReachDoctor(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	if code, _, stderr := dashcrew(t, nil, "sync", "-p", project); code != 0 {
		t.Fatalf("sync: exit %d, %s", code, stderr)
	}

	for _, args := range [][]string{{"guard", "install"}, {"guard", "install", "--expect", "x", "--force"}} {
		if code, stdout, stderr := dashcrew(t, nil, args...); code != 64 || stdout != "" || stderr != guardInstallRemoved+"\n" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	t.Cleanup(func() { stdin = os.Stdin })
	if code, stdout, stderr := dashcrew(t, nil, "guard", filepath.Join(project, "missing.json")); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("guard hook: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	// G4b: sync stamps the git common dir, a malformed payload is logged there, and doctor warns about it.
	common := filepath.Join(project, ".git")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := dashcrew(t, nil, "sync", "-p", project); code != 0 {
		t.Fatalf("sync: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(common, "dashcrew", "last-sync")); err != nil {
		t.Errorf("sync left no stamp: %v", err)
	}
	if _, stdout, _ := dashcrew(t, nil, "doctor", "-p", project); strings.Contains(stdout, "guard-log") {
		t.Errorf("guard-log without a log:\n%s", stdout)
	}
	stdin = strings.NewReader(`{"tool_name":7,"cwd":` + strconv.Quote(project) + `}`)
	if code, stdout, stderr := dashcrew(t, nil, "guard", filepath.Join(project, ".ai", "guard.json")); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("malformed guard call: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	log := filepath.Join(common, "dashcrew", "guard-malformed.log")
	if _, stdout, _ := dashcrew(t, nil, "doctor", "-p", project); !strings.Contains(stdout, "warn  guard-log       the guard let 1 malformed payload through since the last sync, logged in "+log) {
		t.Errorf("doctor:\n%s", stdout)
	}
}

func TestKnowledgeServeAnswersAnMcpClientOnStdio(t *testing.T) {
	clone := filepath.Join(t.TempDir(), "kb")
	writeFile(t, filepath.Join(clone, ".git", "HEAD"), "1111111111111111111111111111111111111111\n")
	writeFile(t, filepath.Join(clone, "features", "share.md"), "---\ntitle: Share\n---\n\nA share link expires after a week.\n")
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	previous := stdin
	stdin = inReader
	t.Cleanup(func() { stdin = previous })

	done := make(chan int)
	var stderr bytes.Buffer
	go func() {
		code := run([]string{"knowledge", "serve", "--clone", clone, "-p", t.TempDir()}, outWriter, &stderr, nil)
		outWriter.Close()
		done <- code
	}()
	responses := bufio.NewReader(outReader)
	send := func(line string) {
		if _, err := io.WriteString(inWriter, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() string {
		line, err := responses.ReadString('\n')
		if err != nil {
			t.Fatalf("%v; stderr %q", err, stderr.String())
		}
		return line
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	if got := receive(); !strings.Contains(got, `"name":"dashcrew-knowledge"`) {
		t.Errorf("initialize %s", got)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"knowledge_search","arguments":{"query":"share link"}}}`)
	if got := receive(); !strings.Contains(got, `"page":"features/share.md"`) {
		t.Errorf("knowledge_search %s", got)
	}
	inWriter.Close()
	if code := <-done; code != 0 {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestKnowledgeServeRefusesAMissingClone(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none")
	code, _, stderr := dashcrew(t, nil, "knowledge", "serve", "--clone", missing)
	if code != 1 || stderr != "No knowledge clone at "+missing+"; run dashcrew knowledge pull.\n" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
