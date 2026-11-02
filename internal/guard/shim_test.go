package guard

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/version"
)

func TestShimIsTheOneTheGoldensRecord(t *testing.T) {
	shim, err := os.ReadFile(filepath.Join("..", "..", "test", "golden", "sample-codex", "expected", ".ai", "bin", "guard"))
	if err != nil {
		t.Fatal(err)
	}
	if Shim(version.GuardMinimum) != string(shim) {
		t.Errorf("the golden shim differs from Shim(%s)", version.GuardMinimum)
	}
}

// posixShells are the POSIX shells on this machine the shim is run with: sh, plus dash when installed.
func posixShells(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the guard shim is a POSIX sh script; Windows is not supported yet")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	out := []string{sh}
	if dash, err := exec.LookPath("dash"); err == nil {
		out = append(out, dash)
	}
	return out
}

// shimFixture is a shim beside a stand-in `dashcrew`, written once per test because macOS checks every new
// executable on its first run. PATH holds only `cat`, `mkdir` and, unless a run has no dashcrew, the stand-in.
type shimFixture struct {
	dir, shim, bin, tools string
}

// The stand-in answers --version per STANDIN_VERSION (printed after `dashcrew `) or, when that is "dart", with the
// Dart usage error. It reads stdin first, which the shim must not hand it. For `guard` it records its arguments
// and stdin, prints a line on each stream and exits with STANDIN_GUARD_EXIT. Each --version call adds a line to
// $STANDIN_DIR/versions.
const standIn = `#!/bin/sh
if [ "$1" = --version ]; then
  cat >/dev/null
  echo called >> "$STANDIN_DIR/versions"
  case $STANDIN_VERSION in
    dart) echo 'Could not find an option named "version".' >&2; exit 64 ;;
    '') exit 0 ;;
  esac
  echo "dashcrew $STANDIN_VERSION"
  exit 0
fi
if [ "$1" = guard ]; then
  shift
  printf '[%s]' "$@" > "$STANDIN_DIR/args"
  cat > "$STANDIN_DIR/stdin"
  echo guard-out
  echo guard-err >&2
  exit "$STANDIN_GUARD_EXIT"
fi
exit 64
`

func (f shimFixture) home() string { return filepath.Join(f.dir, "home") }

// cache is the version cache in the fixture's HOME; ok is false when there is none.
func (f shimFixture) cache(t *testing.T) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.home(), ".dashcrew", "guard-ok"))
	return string(data), err == nil
}

// versionCalls counts the stand-in's --version calls since the run started.
func (f shimFixture) versionCalls() int {
	data, _ := os.ReadFile(filepath.Join(f.dir, "versions"))
	return strings.Count(string(data), "\n")
}

func newShimFixture(t *testing.T) shimFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the guard shim is a POSIX sh script; Windows is not supported yet")
	}
	dir := t.TempDir()
	f := shimFixture{dir: dir, shim: filepath.Join(dir, "guard"), bin: filepath.Join(dir, "bin"), tools: filepath.Join(dir, "tools")}
	for _, folder := range []string{f.bin, f.tools, filepath.Join(dir, "home")} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f.shim, []byte(Shim("0.1.0")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.bin, "dashcrew"), []byte(standIn), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"cat", "mkdir"} {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(found, filepath.Join(f.tools, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// shimRun is one run of the shim. missing leaves the stand-in off PATH; keepCache keeps the version cache of an
// earlier run in the fixture's HOME, which is otherwise deleted first.
type shimRun struct {
	missing   bool
	keepCache bool
	version   string
	guardExit int
	args      []string
	stdin     string
}

type shimResult struct {
	code           int
	stdout, stderr string
	guardCalled    bool
	guardArgs      string
	guardStdin     string
}

func (f shimFixture) run(t *testing.T, shell string, r shimRun) shimResult {
	t.Helper()
	if !r.keepCache {
		if err := os.RemoveAll(filepath.Join(f.home(), ".dashcrew")); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range []string{"args", "stdin", "versions"} {
		if err := os.Remove(filepath.Join(f.dir, record)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	path := f.bin + string(os.PathListSeparator) + f.tools
	if r.missing {
		path = f.tools
	}
	cmd := exec.Command(shell, append([]string{f.shim}, r.args...)...)
	cmd.Env = []string{"PATH=" + path, "HOME=" + f.home(), "STANDIN_DIR=" + f.dir, "STANDIN_VERSION=" + r.version,
		"STANDIN_GUARD_EXIT=" + strconv.Itoa(r.guardExit)}
	cmd.Stdin = strings.NewReader(r.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := shimResult{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	if args, err := os.ReadFile(filepath.Join(f.dir, "args")); err == nil {
		result.guardCalled = true
		result.guardArgs = string(args)
		stdinCopy, _ := os.ReadFile(filepath.Join(f.dir, "stdin"))
		result.guardStdin = string(stdinCopy)
	}
	return result
}

const claudePush = `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git push"}}`

func TestShimRefusesWhenDashcrewIsMissingOrTooOld(t *testing.T) {
	f := newShimFixture(t)
	cases := map[string]shimRun{
		"missing":                {missing: true},
		"dart without --version": {version: "dart"},
		"dart 0.0.1":             {version: "0.0.1"},
		"pre-release of minimum": {version: "0.1.0-dev.1"},
		"older":                  {version: "0.0.9"},
		"two parts":              {version: "1.2"},
		"four parts":             {version: "1.2.3.4"},
		"not a number":           {version: "1.x.0"},
		"no output":              {version: ""},
	}
	for _, shell := range posixShells(t) {
		for name, r := range cases {
			t.Run(filepath.Base(shell)+"/"+name, func(t *testing.T) {
				r.args, r.stdin = []string{"policy.json"}, claudePush
				got := f.run(t, shell, r)

				if got.code != 2 || got.guardCalled || got.stdout != "" {
					t.Fatalf("exit %d, guard called %v, stdout %q, stderr %q", got.code, got.guardCalled, got.stdout, got.stderr)
				}
				if line, ok := f.cache(t); ok {
					t.Errorf("a refusal wrote the cache: %q", line)
				}
				for _, part := range []string{"[dashcrew guard] ", "this tool call is refused", "Install dashcrew 0.1.0 or newer",
					"brew install fahnaladitia/tap/dashcrew"} {
					if !strings.Contains(got.stderr, part) {
						t.Errorf("stderr %q misses %q", got.stderr, part)
					}
				}
				if strings.Count(got.stderr, "\n") != 1 {
					t.Errorf("stderr is not one line: %q", got.stderr)
				}
			})
		}
	}
}

func TestShimNamesTheReason(t *testing.T) {
	f := newShimFixture(t)
	missing := f.run(t, "/bin/sh", shimRun{missing: true, stdin: claudePush})
	old := f.run(t, "/bin/sh", shimRun{version: "0.0.1", stdin: claudePush})

	if !strings.Contains(missing.stderr, "dashcrew is not on PATH, so") {
		t.Errorf("missing: %q", missing.stderr)
	}
	if !strings.Contains(old.stderr, "the dashcrew on PATH is older than 0.1.0 or reports no version, so") {
		t.Errorf("old: %q", old.stderr)
	}
}

func TestShimRefusesAntigravityWithADenyDecision(t *testing.T) {
	f := newShimFixture(t)
	input := `{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}},"workspacePaths":["/w"]}`
	for _, shell := range posixShells(t) {
		got := f.run(t, shell, shimRun{missing: true, stdin: input})

		if got.code != 0 || got.stderr != "" || !strings.HasSuffix(got.stdout, "\"}\n") ||
			!strings.HasPrefix(got.stdout, `{"decision":"deny","reason":"[dashcrew guard] dashcrew is not on PATH`) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", shell, got.code, got.stdout, got.stderr)
		}
	}
}

func TestShimKeepsExitTwoWhenAClaudePayloadMentionsToolCall(t *testing.T) {
	f := newShimFixture(t)
	input := `{"hook_event_name":"PreToolUse","tool_name":"Grep","tool_input":{"pattern":"toolCall"}}`
	got := f.run(t, "/bin/sh", shimRun{missing: true, stdin: input})

	if got.code != 2 || got.stdout != "" {
		t.Errorf("exit %d, stdout %q", got.code, got.stdout)
	}
}

func TestShimRunsDashcrewGuardWithItsArgumentsAndStdin(t *testing.T) {
	f := newShimFixture(t)
	versions := []string{"0.1.0", "0.2.0", "1.0.0", "0.10.0", "0.1.1-0.20261101062238-4b1065d35c59", "0.1.0+abc", "12.0.3"}
	stdin := "{\"tool_input\": {\"command\": \"echo 'a b'\"}}\n\x00\xff last line without newline"
	for _, shell := range posixShells(t) {
		for i, v := range versions {
			guardExit := 2 * (i % 2)
			t.Run(filepath.Base(shell)+"/"+v, func(t *testing.T) {
				got := f.run(t, shell, shimRun{version: v, guardExit: guardExit, args: []string{"/p/my policy.json", "--x", ""}, stdin: stdin})

				if !got.guardCalled || got.code != guardExit {
					t.Fatalf("guard called %v, exit %d (want %d), stderr %q", got.guardCalled, got.code, guardExit, got.stderr)
				}
				if got.guardArgs != "[/p/my policy.json][--x][]" {
					t.Errorf("args %q", got.guardArgs)
				}
				if got.guardStdin != stdin {
					t.Errorf("stdin %q", got.guardStdin)
				}
				if got.stdout != "guard-out\n" || got.stderr != "guard-err\n" {
					t.Errorf("stdout %q, stderr %q", got.stdout, got.stderr)
				}
			})
		}
	}
}

func TestShimCachesAPassedVersionCheck(t *testing.T) {
	for _, shell := range posixShells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			f := newShimFixture(t)
			standIn := filepath.Join(f.bin, "dashcrew")
			cachePath := filepath.Join(f.home(), ".dashcrew", "guard-ok")
			ok := shimRun{version: "0.2.0", args: []string{"p.json"}, stdin: claudePush}
			cached := ok
			cached.keepCache = true
			// fullCheck runs the shim over the cache line content and reports whether it ran --version.
			fullCheck := func(t *testing.T, line string, mode os.FileMode) bool {
				t.Helper()
				writeFileMode(t, cachePath, line, mode)
				got := f.run(t, shell, cached)
				if got.code != 0 || !got.guardCalled {
					t.Fatalf("exit %d, guard called %v, stderr %q", got.code, got.guardCalled, got.stderr)
				}
				return f.versionCalls() == 1
			}

			got := f.run(t, shell, ok)
			line, written := f.cache(t)
			if got.code != 0 || !got.guardCalled || f.versionCalls() != 1 || !written || line != standIn+" 0.1.0\n" {
				t.Fatalf("miss: exit %d, versions %d, cache %q %v", got.code, f.versionCalls(), line, written)
			}

			got = f.run(t, shell, cached)
			if got.code != 0 || f.versionCalls() != 0 || got.guardArgs != "[p.json]" || got.guardStdin != claudePush {
				t.Errorf("hit: exit %d, versions %d, args %q, stdin %q", got.code, f.versionCalls(), got.guardArgs, got.guardStdin)
			}

			for name, line := range map[string]string{
				"another path": filepath.Join(f.dir, "other", "dashcrew") + " 0.1.0\n",
				"another need": standIn + " 0.0.9\n",
				"corrupt":      "\x00\xff garbage",
				"empty":        "",
				"no newline":   standIn + " 0.1.0",
			} {
				if name == "no newline" {
					continue // read returns false at EOF without a newline, so this is a miss as well; asserted below
				}
				if !fullCheck(t, line, 0o644) {
					t.Errorf("%s: the cache was trusted", name)
				}
			}
			if !fullCheck(t, standIn+" 0.1.0", 0o644) {
				t.Error("no newline: the cache was trusted")
			}
			if os.Geteuid() != 0 && !fullCheck(t, standIn+" 0.1.0\n", 0o000) {
				t.Error("unreadable: the cache was trusted")
			}
			if err := os.Chmod(cachePath, 0o644); err != nil {
				t.Fatal(err)
			}

			writeFileMode(t, cachePath, standIn+" 0.1.0\n", 0o644)
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(standIn, later, later); err != nil {
				t.Fatal(err)
			}
			if !fullCheck(t, standIn+" 0.1.0\n", 0o644) {
				// writeFileMode rewrote the cache now, an hour before the binary's time
				t.Error("a binary newer than the cache: the cache was trusted")
			}

			old := shimRun{version: "0.0.1", keepCache: true, stdin: claudePush}
			writeFileMode(t, cachePath, standIn+" 0.0.9\n", 0o644)
			if got := f.run(t, shell, old); got.code != 2 || got.guardCalled {
				t.Errorf("a stale line let an old dashcrew through: exit %d", got.code)
			}
			if line, _ := f.cache(t); line != standIn+" 0.0.9\n" {
				t.Errorf("a refusal rewrote the cache: %q", line)
			}
		})
	}
}

func TestShimWithoutHomeChecksEveryTimeAndWritesNothing(t *testing.T) {
	f := newShimFixture(t)
	for i := 0; i < 2; i++ {
		cmd := exec.Command("/bin/sh", f.shim)
		cmd.Env = []string{"PATH=" + f.bin + string(os.PathListSeparator) + f.tools, "STANDIN_DIR=" + f.dir,
			"STANDIN_VERSION=0.1.0", "STANDIN_GUARD_EXIT=0"}
		cmd.Stdin = strings.NewReader(claudePush)
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	if f.versionCalls() != 2 {
		t.Errorf("versions %d", f.versionCalls())
	}
	if _, err := os.Stat(filepath.Join(f.dir, ".dashcrew")); err == nil {
		t.Error("cache written relative to an empty HOME")
	}
}

func writeFileMode(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(path, 0o644)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
