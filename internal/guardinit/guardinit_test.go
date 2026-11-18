package guardinit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

func read(t *testing.T, dir, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, dir, relative, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, force bool, agents ...string) []string {
	t.Helper()
	written, err := Init{ProjectDir: dir, Agents: agents, Force: force}.Run()
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func TestInitWritesThePolicyTheShimAndEachHook(t *testing.T) {
	dir := t.TempDir()

	written := run(t, dir, false, "claude-code", "antigravity")

	if got, want := strings.Join(written, " "), ".ai/guard.json .ai/bin/guard .claude/settings.json .agents/hooks.json"; got != want {
		t.Errorf("wrote %s, want %s", got, want)
	}
	if read(t, dir, guard.InitPolicyPath) != guard.InitPolicyText() {
		t.Error("the policy is not the default one")
	}
	if got := read(t, dir, guard.ShimPath); got != guard.Shim(version.AntigravityGuardMinimum) {
		t.Errorf("the shim does not need the antigravity minimum:\n%s", got)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(dir, guard.ShimPath)); info.Mode()&0o111 == 0 {
			t.Error("the shim is not executable")
		}
	}
	if !strings.Contains(read(t, dir, ".claude/settings.json"), ".ai/bin/guard") {
		t.Error("no Claude Code hook")
	}
	if again := run(t, dir, false, "claude-code", "antigravity"); len(again) != 0 {
		t.Errorf("a second run wrote %v", again)
	}
}

func TestInitKeepsTheHighestMinimumOfAnEarlierRun(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, false, "antigravity")

	run(t, dir, false, "claude-code")

	if got := guard.ShimMinimum(read(t, dir, guard.ShimPath)); got != version.AntigravityGuardMinimum {
		t.Errorf("the shim needs %s; the antigravity hook of the first run needs %s", got, version.AntigravityGuardMinimum)
	}
}

func TestInitRefuses(t *testing.T) {
	edited := strings.Replace(guard.InitPolicyText(), `"merge",`, "", 1)
	cases := []struct {
		name  string
		setup func(dir string)
		agent string
		want  string
	}{
		{"a profile", func(dir string) { write(t, dir, ".ai/project.yaml", "toolkit: 1\n") }, "claude-code",
			"This project has .ai/project.yaml: dashcrew sync writes and updates its guard."},
		{"an edited policy", func(dir string) { write(t, dir, guard.InitPolicyPath, edited) }, "claude-code",
			"Not written by dashcrew guard init (rerun with --force to overwrite):\n  .ai/guard.json"},
		{"another shim", func(dir string) { write(t, dir, guard.ShimPath, "#!/bin/sh\n") }, "claude-code",
			"Not written by dashcrew guard init (rerun with --force to overwrite):\n  .ai/bin/guard"},
		{"an unknown agent", func(string) {}, "cursor", "Agent adapter not available yet: cursor."},
	}
	for _, c := range cases {
		dir := t.TempDir()
		c.setup(dir)
		_, err := Init{ProjectDir: dir, Agents: []string{c.agent}}.Run()
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".claude/settings.json")); statErr == nil {
			t.Errorf("%s: a hook was written before the refusal", c.name)
		}
	}
}

func TestForceReplacesAnEditedPolicy(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, guard.InitPolicyPath, "{}\n")

	run(t, dir, true, "claude-code")

	if read(t, dir, guard.InitPolicyPath) != guard.InitPolicyText() {
		t.Error("--force did not write the default policy")
	}
}
