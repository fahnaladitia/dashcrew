package shell

import (
	"bufio"
	"os"
	"os/exec"
	"reflect"
	stdsync "sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
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
}

// dartOracle is a running Dart `shellCommands` (testdata/shell_oracle.dart, compiled to the path in
// DASHCREW_DART_SHELL); nil without it, and the fuzz target then checks the Go properties only.
type dartOracle struct {
	mu  stdsync.Mutex
	in  *bufio.Writer
	out *bufio.Scanner
}

var (
	oracleOnce stdsync.Once
	oracle     *dartOracle
)

func startOracle(t testing.TB) *dartOracle {
	oracleOnce.Do(func() {
		path := os.Getenv("DASHCREW_DART_SHELL")
		if path == "" {
			return
		}
		cmd := exec.Command(path)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1<<20), 1<<26)
		oracle = &dartOracle{in: bufio.NewWriter(stdin), out: scanner}
	})
	return oracle
}

// commands asks Dart for the commands of script, as JSON.
func (o *dartOracle) commands(t *testing.T, script string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	line, err := dartjson.Encode(script)
	if err != nil {
		t.Fatal(err)
	}
	o.in.WriteString(line + "\n")
	if err := o.in.Flush(); err != nil {
		t.Fatal(err)
	}
	answer := make(chan string, 1)
	go func() {
		if o.out.Scan() {
			answer <- o.out.Text()
		}
		close(answer)
	}()
	select {
	case text, ok := <-answer:
		if !ok {
			t.Fatalf("Dart oracle stopped on %q: %v", script, o.out.Err())
		}
		return text
	case <-time.After(10 * time.Second):
		t.Fatalf("Dart oracle gave no answer in 10 s for %q", script)
	}
	return ""
}

func commandsJSON(t *testing.T, commands [][]string) string {
	list := make([]any, len(commands))
	for i, argv := range commands {
		words := make([]any, len(argv))
		for j, word := range argv {
			words[j] = word
		}
		list[i] = words
	}
	text, err := dartjson.Encode(list)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// FuzzCommands checks that Commands never panics, returns only non-empty commands, is deterministic, and, with
// the Dart oracle, returns exactly what Dart's shellCommands returns for every valid UTF-8 script.
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
		if o := startOracle(t); o != nil && utf8.ValidString(script) {
			if want, have := o.commands(t, script), commandsJSON(t, got); want != have {
				t.Fatalf("script %q\n dart: %s\n   go: %s", script, want, have)
			}
		}
	})
}
