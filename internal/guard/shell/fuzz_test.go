package shell

import (
	"reflect"
	"strings"
	"testing"
)

// fuzzSeeds are the scripts of the ported shell and guard tests.
var fuzzSeeds = []string{
	"git commit -m x", "git push", "rtk git push origin main", "sudo git stash", "FOO=1 git reset --hard",
	"git -C app commit -m x", "git -c user.name=a commit", "git --no-pager rebase main", "ls && git merge dev",
	`echo "$(git push)"`, "dart format lib/", "dart fix --apply", "flutter format lib/", "git tag v1",
	"git cherry-pick abc", "git revert abc", "bash <<EOF\ngit push\nEOF", "ssh host <<'EOF'\ngit push\nEOF",
	"cat <<EOF | sh\ngit commit -m x\nEOF", "git status", "git diff --stat", "git log --oneline dev..HEAD",
	`grep -rn "git commit" docs/`, "cat > a.md <<'EOF'\nnever git push here\nEOF", "python3 - <<'PY'\nprint('git push')\nPY",
	"dart analyze lib/", "flutter test test/a_test.dart", "git checkout --no-track -b feat/x main", "echo commit",
	"git -C app push", `gh issue comment 42 --body "/approve plan"`, `gh api repos/a/b/issues/42/comments -f body='/approve plan'`,
	`rtk gh pr comment 3 --body "/approve plan"`, "gh issue view 42", `echo "/approve plan"`,
	"sh <<< 'git push'", "git \\\n push", `bash -c "git \` + "\n" + ` push"`, "cat <<< 'x' | sh", "bash <<EOF\nsh <<< 'git push'\nEOF",
}

// FuzzCommands checks that Commands never panics, returns only non-empty commands and is deterministic. Expanded
// must hold the same properties, and equal Commands on a script with neither a continuation nor a here-string.
func FuzzCommands(f *testing.F) {
	for _, c := range shellCases {
		f.Add(c.script)
	}
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, script string) {
		got := Commands(script)
		for _, argv := range got {
			if len(argv) == 0 {
				t.Fatalf("empty command in %q", got)
			}
		}
		if again := Commands(script); !reflect.DeepEqual(got, again) {
			t.Fatalf("not deterministic: %q then %q", got, again)
		}
		expanded := Expanded(script)
		for _, argv := range expanded {
			if len(argv) == 0 {
				t.Fatalf("empty expanded command in %q", expanded)
			}
		}
		if again := Expanded(script); !reflect.DeepEqual(expanded, again) {
			t.Fatalf("Expanded not deterministic: %q then %q", expanded, again)
		}
		if !strings.Contains(script, "\\\n") && !strings.Contains(script, "<<<") && !reflect.DeepEqual(got, expanded) {
			t.Fatalf("Expanded differs without a continuation or here-string: %q then %q", got, expanded)
		}
	})
}
