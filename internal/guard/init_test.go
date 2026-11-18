package guard

import (
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

func TestInitPolicyIsTheDefaultProfileWithoutRoles(t *testing.T) {
	fromProfile := compiled(t, goOther)
	fromProfile.RoleCapability = map[string]string{}
	want, _ := dartjson.EncodeIndent(fromProfile.ToJSON(), "  ")
	got, _ := dartjson.EncodeIndent(InitPolicy().ToJSON(), "  ")
	if got != want {
		t.Errorf("InitPolicy:\n%s\nwant what a platform: other profile compiles to, without roles:\n%s", got, want)
	}
	if InitPolicyText() != got+"\n" {
		t.Error("InitPolicyText is not the indented policy with a final newline")
	}
}

func TestInitPolicyBlocksHistoryAndAllowsReading(t *testing.T) {
	p := InitPolicy()
	for _, command := range []string{"git push", "git commit -m x", `bash -c "git reset --hard"`, "env FOO=1 git rebase main", "dart format ."} {
		if !blocked(t, shellEvent(command), p) {
			t.Errorf("%q is allowed", command)
		}
	}
	for _, command := range []string{"git status", "git diff", "go test ./..."} {
		if blocked(t, shellEvent(command), p) {
			t.Errorf("%q is blocked", command)
		}
	}
	if p.Minimum() != version.GuardMinimum {
		t.Errorf("Minimum = %s, want %s", p.Minimum(), version.GuardMinimum)
	}
}

func TestWrittenByInit(t *testing.T) {
	shim := Shim(version.AntigravityGuardMinimum)
	cases := []struct {
		path, content string
		want          bool
	}{
		{InitPolicyPath, InitPolicyText(), true},
		{InitPolicyPath, strings.Replace(InitPolicyText(), `"merge",`, "", 1), false},
		{InitPolicyPath, "", false},
		{ShimPath, Shim(version.GuardMinimum), true},
		{ShimPath, shim, true},
		{ShimPath, shim + "# mine\n", false},
		{ShimPath, strings.Replace(shim, "exit 2", "exit 0", 1), false},
		{ShimPath, "#!/bin/sh\nexec dashcrew guard \"$@\"\n", false},
		{"AGENTS.md", InitPolicyText(), false},
	}
	for _, c := range cases {
		if got := WrittenByInit(c.path, c.content); got != c.want {
			t.Errorf("WrittenByInit(%s, %.40q) = %v, want %v", c.path, c.content, got, c.want)
		}
	}
}
