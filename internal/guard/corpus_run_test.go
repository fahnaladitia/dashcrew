package guard

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
)

// corpusFolders holds a project with the default policy, the named policies, and a folder without a policy.
type corpusFolders struct {
	project, policies, elsewhere string
}

func newCorpusFolders(t testing.TB) corpusFolders {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := corpusFolders{
		project:   filepath.Join(root, "project"),
		policies:  filepath.Join(root, "policies"),
		elsewhere: filepath.Join(root, "elsewhere"),
	}
	for _, dir := range []string{filepath.Join(f.project, ".ai"), filepath.Join(f.project, "lib"), f.policies, f.elsewhere} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range corpusPolicies {
		if err := os.WriteFile(filepath.Join(f.policies, name+".json"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.project, ".ai", "guard.json"), []byte(corpusPolicies["default"]), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f corpusFolders) expand(text string) string {
	return strings.NewReplacer("{project}", f.project, "{policies}", f.policies).Replace(text)
}

// collapse is expand reversed, so recorded outcomes do not name the temp folders.
func (f corpusFolders) collapse(text string) string {
	return strings.NewReplacer(f.project, "{project}", f.policies, "{policies}", f.elsewhere, "{elsewhere}").Replace(text)
}

type outcome struct {
	exit           int
	stdout, stderr string
}

func (o outcome) String() string {
	return fmt.Sprintf("exit %d, stdout %q, stderr %q", o.exit, o.stdout, o.stderr)
}

// runProcess runs one corpus case as a process: argv then the case's arguments.
func runProcess(f corpusFolders, c corpusCase, argv ...string) (outcome, error) {
	for _, arg := range c.args {
		argv = append(argv, f.expand(arg))
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = f.elsewhere
	if c.inProject {
		cmd.Dir = f.project
	}
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + f.elsewhere}
	for key, value := range c.env {
		cmd.Env = append(cmd.Env, key+"="+f.expand(value))
	}
	cmd.Stdin = strings.NewReader(f.expand(c.stdin))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return outcome{exit.ExitCode(), stdout.String(), stderr.String()}, nil
	}
	return outcome{0, stdout.String(), stderr.String()}, err
}

// runInProcess runs one corpus case through Run with the environment and working folder runProcess gives it.
func runInProcess(f corpusFolders, c corpusCase) outcome {
	args := make([]string, len(c.args))
	for i, arg := range c.args {
		args[i] = f.expand(arg)
	}
	dir := f.elsewhere
	if c.inProject {
		dir = f.project
	}
	env := Env{
		LookupEnv: func(key string) (string, bool) {
			value, ok := c.env[key]
			return f.expand(value), ok
		},
		Getwd: func() (string, error) { return dir, nil },
	}
	var stdout, stderr bytes.Buffer
	code := Run(args, strings.NewReader(f.expand(c.stdin)), &stdout, &stderr, env)
	return outcome{code, stdout.String(), stderr.String()}
}

// buildDashcrew builds cmd/dashcrew into a temporary folder.
func buildDashcrew(t testing.TB) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "dashcrew")
	build := exec.Command("go", "build", "-o", binary, "./cmd/dashcrew")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binary
}

// corpusRecordPath records every corpus case the guard answers with an exit code or output: one line per case,
// its name then the exit code, stdout and stderr as JSON strings, with the temp folders written as {project},
// {policies} and {elsewhere}. Every case not listed is allowed silently (exit 0, no output).
//
// It was recorded at G8 while `dashcrew guard` matched the Dart guard on every case Dart blocks (exit code,
// stdout and stderr), with the Go-only blocks of G4b listed in testdata/corpus_new_blocks.txt. So it keeps the
// differential guarantee without Dart: the guard still blocks what Dart blocked, with the same answer.
const corpusRecordPath = "testdata/corpus_outcomes.txt"

func corpusRecordLine(name string, o outcome) string {
	return strings.Join([]string{name, strconv.Itoa(o.exit), jsonString(o.stdout), jsonString(o.stderr)}, "\t")
}

func readCorpusRecord(t *testing.T) map[string]outcome {
	t.Helper()
	file, err := os.Open(corpusRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	out := map[string]outcome{}
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	for lines.Scan() {
		if lines.Text() == "" || strings.HasPrefix(lines.Text(), "#") {
			continue
		}
		fields := strings.Split(lines.Text(), "\t")
		if len(fields) != 4 {
			t.Fatalf("%s: bad line %q", corpusRecordPath, lines.Text())
		}
		exit, err := strconv.Atoi(fields[1])
		stdout, errOut := dartjson.Decode(fields[2])
		stderr, errErr := dartjson.Decode(fields[3])
		stdoutText, okOut := stdout.(string)
		stderrText, okErr := stderr.(string)
		if err != nil || errOut != nil || errErr != nil || !okOut || !okErr {
			t.Fatalf("%s: bad line %q", corpusRecordPath, lines.Text())
		}
		if _, ok := out[fields[0]]; ok {
			t.Fatalf("%s: duplicate case %q", corpusRecordPath, fields[0])
		}
		out[fields[0]] = outcome{exit, stdoutText, stderrText}
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCorpusOutcomesMatchTheRecord runs every corpus case in process and compares it with the record. Rewrite the
// record after an intended guard change with `DASHCREW_UPDATE_GOLDENS=1 go test ./internal/guard -run
// CorpusOutcomes`, then review the diff: a line that disappears or changes is a case the guard no longer blocks
// the same way.
func TestCorpusOutcomesMatchTheRecord(t *testing.T) {
	corpus := guardCorpus()
	if len(corpus) < 13000 {
		t.Fatalf("corpus has %d cases, want at least 13000", len(corpus))
	}
	f := newCorpusFolders(t)
	names := map[string]bool{}
	var answered []string
	got := map[string]outcome{}
	for _, c := range corpus {
		if names[c.name] {
			t.Fatalf("duplicate case name %q", c.name)
		}
		names[c.name] = true
		o := runInProcess(f, c)
		o.stdout, o.stderr = f.collapse(o.stdout), f.collapse(o.stderr)
		if o != (outcome{}) {
			answered = append(answered, c.name)
			got[c.name] = o
		}
	}

	if os.Getenv("DASHCREW_UPDATE_GOLDENS") == "1" {
		var b strings.Builder
		b.WriteString("# Every guard corpus case answered with an exit code or output (see corpusRecordPath).\n")
		b.WriteString("# name, exit code, stdout and stderr as JSON strings; unlisted cases are allowed silently.\n")
		for _, name := range answered {
			b.WriteString(corpusRecordLine(name, got[name]) + "\n")
		}
		if err := os.WriteFile(corpusRecordPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	recorded := readCorpusRecord(t)
	changed := 0
	for _, c := range corpus {
		want, inRecord := recorded[c.name]
		have := got[c.name]
		if want != have {
			changed++
			if changed <= 20 {
				t.Errorf("%s\nstdin %q args %q\n recorded: %s\n      now: %s", c.name, c.stdin, c.args, want, have)
			}
		}
		if inRecord {
			delete(recorded, c.name)
		}
	}
	for name := range recorded {
		t.Errorf("%s: recorded but no longer in the corpus", name)
	}
	if changed > 0 {
		t.Errorf("%d cases differ from %s", changed, corpusRecordPath)
	}
	t.Logf("corpus: %d cases, %d answered", len(corpus), len(answered))
}
