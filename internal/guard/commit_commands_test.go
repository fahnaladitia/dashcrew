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

// Release lines (P6): recording a branch's release is local branch config, and the release and port checks only read
// git, so the agent may run them; the merge or cherry-pick a port needs stays the user's.
func TestReleaseCommandsAreAllowedWhileMergeAndCherryPickStayBlocked(t *testing.T) {
	for _, p := range []Policy{testPolicy(false, false), testPolicy(true, false)} {
		for _, command := range []string{
			"git config branch.fix/totals.dashcrew-release 2.4.5",
			"git config --get branch.fix/totals.dashcrew-release",
			"dashcrew release lines",
			"dashcrew release status 2.4.5",
			"dashcrew check --ports --from 2.4.5 --to main",
			"dashcrew commit suggest --ported-from abc1234 --subject 'keep the total'",
		} {
			if blocked(t, shellEvent(command), p) {
				t.Errorf("blocked: %s", command)
			}
		}
		for _, command := range []string{
			"git " + "merge --no-ff --no-commit -X find-renames release/2.4.5",
			"git " + "cherry-pick -x --no-commit abc1234",
		} {
			if !blocked(t, shellEvent(command), p) {
				t.Errorf("allowed: %s", command)
			}
		}
	}
}
