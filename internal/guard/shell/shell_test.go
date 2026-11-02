package shell

import (
	"reflect"
	"testing"
)

// dartShellTests is the number of tests the Dart shell_test.dart had (removed at G8).
// Every one of them is a row of shellCases.
const dartShellTests = 9

type shellCase struct {
	dart   string // the Dart test name
	script string
	// exact is the whole result, in order; contains lists commands that must appear, in any order.
	exact    [][]string
	contains [][]string
}

var shellCases = []shellCase{
	{
		dart:   "splits on separators and keeps argv words",
		script: "git status && flutter test test/a_test.dart; ls | wc -l",
		exact:  [][]string{{"git", "status"}, {"flutter", "test", "test/a_test.dart"}, {"ls"}, {"wc", "-l"}},
	},
	{
		dart:   "quoted text is one data word, never a command",
		script: `grep -rn "git commit" docs/`,
		exact:  [][]string{{"grep", "-rn", "git commit", "docs/"}},
	},
	{
		dart:   "quoted text is one data word, never a command",
		script: `echo 'a; git push'`,
		exact:  [][]string{{"echo", "a; git push"}},
	},
	{
		dart:     "command substitution is a command, also inside double quotes",
		script:   `echo "$(git push origin)"`,
		contains: [][]string{{"git", "push", "origin"}},
	},
	{
		dart:     "command substitution is a command, also inside double quotes",
		script:   "x=`git stash`",
		contains: [][]string{{"git", "stash"}},
	},
	{
		dart:   "heredoc body is data",
		script: "cat > notes.md <<'EOF'\nrun git commit later\ndart format is banned\nEOF\nls",
		exact:  [][]string{{"cat"}, {"ls"}},
	},
	{
		dart:   "unquoted heredoc body is data too",
		script: "cat <<-EOF > a.md\n\tgit push\n\tEOF\necho done",
		exact:  [][]string{{"cat"}, {"echo", "done"}},
	},
	{
		dart:     "heredoc fed to a shell on its line is parsed as commands",
		script:   "bash <<EOF\ngit push\nEOF\nls",
		contains: [][]string{{"bash"}, {"git", "push"}, {"ls"}},
	},
	{
		dart:     "heredoc fed to a shell on its line is parsed as commands",
		script:   "cat <<'EOF' | /bin/sh\ndart format lib\nEOF",
		contains: [][]string{{"dart", "format", "lib"}},
	},
	{
		dart:   "redirect targets are not argv words",
		script: "flutter test > out.txt 2>&1",
		exact:  [][]string{{"flutter", "test"}},
	},
	{
		dart:   "subshell parentheses split commands",
		script: "(cd app && git commit -m x)",
		exact:  [][]string{{"cd", "app"}, {"git", "commit", "-m", "x"}},
	},
	{
		dart:   "escaped characters stay in the word",
		script: `echo a\;b`,
		exact:  [][]string{{"echo", "a;b"}},
	},
}

func TestEveryDartShellTestIsPorted(t *testing.T) {
	names := map[string]bool{}
	for _, c := range shellCases {
		names[c.dart] = true
	}
	if len(names) != dartShellTests {
		t.Errorf("%d Dart shell tests ported, want %d", len(names), dartShellTests)
	}
	t.Logf("shell_test.dart: %d tests ported as %d cases", len(names), len(shellCases))
}

func TestCommands(t *testing.T) {
	for _, c := range shellCases {
		t.Run(c.dart, func(t *testing.T) {
			got := Commands(c.script)
			if c.exact != nil && !reflect.DeepEqual(got, c.exact) {
				t.Errorf("Commands(%q) = %q, want %q", c.script, got, c.exact)
			}
			for _, want := range c.contains {
				found := false
				for _, argv := range got {
					if reflect.DeepEqual(argv, want) {
						found = true
					}
				}
				if !found {
					t.Errorf("Commands(%q) = %q, missing %q", c.script, got, want)
				}
			}
		})
	}
}

func TestScanListsOutputRedirectionTargets(t *testing.T) {
	for script, want := range map[string][]string{
		"echo x > a":                         {"a"},
		"echo x >> 'b c' 2>&1 < in":          {"b c"},
		"cmd &> d; cmd >| e; cmd 1>f 2>&-":   {"d", "e", "f"},
		"cmd <> g; cmd >&h":                  {"g", "h"},
		"echo $(echo y > i) `echo z >> j`":   {"i", "j"},
		"bash <<EOF\necho x > k\nEOF":        {"k"},
		"cat <<EOF > l\nnot > m\nEOF":        {"l"},
		"sh <<< 'echo x > n'":                {"n"},
		"echo x < o; cat <<< p; echo q 2>&1": nil,
	} {
		_, got := Scan(script)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", script, got, want)
		}
	}
	commands, _ := Scan("a > x | b")
	if !reflect.DeepEqual(commands, Expanded("a > x | b")) {
		t.Errorf("Scan commands %q differ from Expanded", commands)
	}
}
