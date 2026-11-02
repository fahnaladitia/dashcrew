package guard

import "testing"

// The commit commands only read git (P5b), so an agent may run them while git commit and push stay the user's.
func TestCommitCommandsAreAllowedWhileGitCommitStaysBlocked(t *testing.T) {
	p := testPolicy(false, false)
	for _, command := range []string{
		"dashcrew commit suggest",
		`dashcrew commit suggest --subject "keep the total after git commit" --body "Why."`,
		"dashcrew commit suggest --json -p app",
		`dashcrew commit lint "fix(cart): keep the total"`,
		"dashcrew commit lint --range main..HEAD",
		"git log -1 --format=%B | dashcrew commit lint -",
		"dashcrew verify record",
		"rtk dashcrew commit suggest",
	} {
		if blocked(t, shellEvent(command), p) {
			t.Errorf("blocked: %s", command)
		}
	}
	for _, command := range []string{
		"git commit -m 'fix(cart): keep the total'",
		"git add -- a.dart && git commit -m 'fix: x'",
		"dashcrew commit suggest && git push",
	} {
		if !blocked(t, shellEvent(command), p) {
			t.Errorf("allowed: %s", command)
		}
	}
	if blocked(t, shellEvent("git commit -m 'fix: x'"), testPolicy(true, false)) {
		t.Error("agent_commit: true blocks git commit")
	}
}
