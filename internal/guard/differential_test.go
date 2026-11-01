package guard

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	stdsync "sync"
	"testing"
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

// dartGuard is the compiled Dart guard named by DASHCREW_DART_GUARD; the test skips without it. Build it with
// `dart compile exe bin/guard.dart -o <path>` from the repository root.
func dartGuard(t testing.TB) string {
	t.Helper()
	path := os.Getenv("DASHCREW_DART_GUARD")
	if path == "" {
		t.Skip("DASHCREW_DART_GUARD is not set: no compiled Dart guard to compare with")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("Dart guard %s: %v", path, err)
	}
	return path
}

func TestCorpusIsLargeAndRunsInProcess(t *testing.T) {
	corpus := guardCorpus()
	if len(corpus) < 2000 {
		t.Fatalf("corpus has %d cases, want at least 2000", len(corpus))
	}
	f := newCorpusFolders(t)
	blocked := 0
	for _, c := range corpus {
		if c.env != nil || c.inProject {
			continue
		}
		args := make([]string, len(c.args))
		for i, arg := range c.args {
			args[i] = f.expand(arg)
		}
		var stdout, stderr bytes.Buffer
		env := Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return f.elsewhere, nil }}
		if Run(args, strings.NewReader(f.expand(c.stdin)), &stdout, &stderr, env) == 2 || stdout.Len() > 0 {
			blocked++
		}
	}
	t.Logf("corpus: %d cases, %d blocked in process", len(corpus), blocked)
}

// TestGuardMatchesDartGuard runs every corpus case through the Dart guard and `dashcrew guard` and requires the
// same exit code, stdout and stderr.
func TestGuardMatchesDartGuard(t *testing.T) {
	dart := dartGuard(t)
	goBinary := buildDashcrew(t)
	f := newCorpusFolders(t)
	corpus := guardCorpus()

	type result struct {
		c           corpusCase
		dart, goOut outcome
		err         error
	}
	results := make([]result, len(corpus))
	jobs := make(chan int)
	var wg stdsync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				c := corpus[i]
				d, errDart := runProcess(f, c, dart)
				g, errGo := runProcess(f, c, goBinary, "guard")
				results[i] = result{c: c, dart: d, goOut: g, err: errors.Join(errDart, errGo)}
			}
		}()
	}
	for i := range corpus {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	same, blocked, denied := 0, 0, 0
	for _, r := range results {
		if r.err != nil {
			t.Errorf("%s: %v", r.c.name, r.err)
			continue
		}
		if r.dart != r.goOut {
			t.Errorf("%s\nstdin %q args %q\n dart: %s\n   go: %s", r.c.name, r.c.stdin, r.c.args, r.dart, r.goOut)
			continue
		}
		same++
		if r.dart.exit == 2 {
			blocked++
		}
		if r.dart.stdout != "" {
			denied++
		}
	}
	t.Logf("%d of %d cases identical; %d blocked with exit 2, %d denied on stdout", same, len(corpus), blocked, denied)
}
