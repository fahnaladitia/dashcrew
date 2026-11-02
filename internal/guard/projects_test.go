package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// appPolicy forbids package:vendor_sdk/ and dart format, and blocks git commit.
	appPolicy = `{"schema":1,"blocked_git":["commit","push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{"reviewer":"read"},"forbidden_imports":["package:vendor_sdk/"],"local_packages":[]}`
	// corePolicy forbids package:http/ only, allows formatters and git commit.
	corePolicy = `{"schema":1,"blocked_git":["push"],"block_formatters":false,"plans_dir":".ai/plans","role_capability":{"reviewer":"read"},"forbidden_imports":["package:http/"],"local_packages":[]}`
)

// twoProjects is a workspace folder holding app and core, each with its own policy, app linking core as
// packages/core; it returns the workspace, app and core folders.
func twoProjects(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app, core := filepath.Join(root, "app"), filepath.Join(root, "core")
	for dir, policy := range map[string]string{app: appPolicy, core: corePolicy} {
		if err := os.MkdirAll(filepath.Join(dir, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".ai", "guard.json"), []byte(policy), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(app, "packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(core, filepath.Join(app, "packages", "core")); err != nil {
		t.Skip("symbolic links are not available: " + err.Error())
	}
	return root, app, core
}

// guardIn runs the guard for payload as a hook in cwd; policy is the explicit policy argument, empty for none.
func guardIn(t *testing.T, cwd, policy, payload string) (bool, string) {
	t.Helper()
	env := Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return cwd, nil }}
	var args []string
	if policy != "" {
		args = []string{policy}
	}
	var stdout, stderr bytes.Buffer
	code := Run(args, strings.NewReader(payload), &stdout, &stderr, env)
	return code == 2, stderr.String()
}

func dartWrite(path, imported string) string {
	return payloadJSON(map[string]any{"tool_name": "Write", "tool_input": map[string]any{
		"file_path": path, "content": "import '" + imported + "x.dart';\n"}})
}

func TestALinkedProjectIsCheckedUnderItsOwnPolicy(t *testing.T) {
	_, app, core := twoProjects(t)
	appGuard := filepath.Join(app, ".ai", "guard.json")
	cases := []struct {
		name, path, imported string
		blocked              bool
		reason               string
	}{
		{"app rule in app", filepath.Join(app, "lib", "a.dart"), "package:vendor_sdk/", true, "vendor_sdk"},
		{"core rule in core, absolute", filepath.Join(core, "lib", "a.dart"), "package:http/", true, "package:http/"},
		{"core rule through the link", filepath.Join("packages", "core", "lib", "a.dart"), "package:http/", true, "package:http/"},
		{"app rule does not reach core", filepath.Join(core, "lib", "a.dart"), "package:vendor_sdk/", false, ""},
		{"app rule does not reach core through the link", filepath.Join("packages", "core", "lib", "a.dart"), "package:vendor_sdk/", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			blocked, reason := guardIn(t, app, appGuard, dartWrite(c.path, c.imported))
			if blocked != c.blocked || !strings.Contains(reason, c.reason) {
				t.Errorf("blocked %v %q, want %v %q", blocked, reason, c.blocked, c.reason)
			}
		})
	}
}

func TestAWorkspaceFolderUsesThePoliciesBelowIt(t *testing.T) {
	root, _, core := twoProjects(t)
	missing := filepath.Join(root, ".ai", "guard.json")
	outside := t.TempDir()
	cases := []struct {
		name, payload string
		blocked       bool
	}{
		{"git push, blocked by both", bashPayload("git push"), true},
		{"git commit, blocked by app only", bashPayload("cd core && git commit -m x"), true},
		{"dart format, blocked by app only", bashPayload("dart format core"), true},
		{"git status", bashPayload("git status"), false},
		{"app file under app rules", dartWrite(filepath.Join("app", "lib", "a.dart"), "package:vendor_sdk/"), true},
		{"core file under core rules", dartWrite(filepath.Join(core, "lib", "a.dart"), "package:http/"), true},
		{"core file is not under app rules", dartWrite(filepath.Join(core, "lib", "a.dart"), "package:vendor_sdk/"), false},
		{"a file outside every project", dartWrite(filepath.Join(outside, "lib", "a.dart"), "package:vendor_sdk/"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, policy := range []string{missing, ""} {
				if blocked, reason := guardIn(t, root, policy, c.payload); blocked != c.blocked {
					t.Errorf("policy %q: blocked %v %q, want %v", policy, blocked, reason, c.blocked)
				}
			}
		})
	}
}

func TestAFolderWithoutProjectsBelowStaysAllowed(t *testing.T) {
	dir := t.TempDir()
	if blocked, _ := guardIn(t, dir, "", bashPayload("git push")); blocked {
		t.Error("a folder with no project is guarded")
	}
}

func TestAPolicyOutsideAProjectKeepsOnePolicy(t *testing.T) {
	_, app, core := twoProjects(t)
	external := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(external, []byte(appPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if blocked, _ := guardIn(t, app, external, dartWrite(filepath.Join(core, "lib", "a.dart"), "package:http/")); blocked {
		t.Error("an external policy picked up the core project's rules")
	}
}
