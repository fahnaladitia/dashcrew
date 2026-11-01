package guard

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
)

// G4b: what the Go guard blocks beyond the Dart guard. Every Go-only block of the differential corpus (run against
// the Dart guard until G8) is a row of newBlocks or a line of testdata/corpus_new_blocks.txt.

// newBlock is a shell command the Dart guard allows and the Go guard blocks, with the gap it closes (the "Known
// gaps" of the G4 plan) and a fragment of the expected reason.
type newBlock struct {
	gap     int
	command string
	reason  string
}

const (
	pushReason    = "git push is reserved"
	commitReason  = "git commit is reserved"
	stashReason   = "git stash is reserved"
	resetReason   = "git reset is reserved"
	tagReason     = "git tag is reserved"
	formatReason  = "dart format rewrites"
	approveReason = "Approvals are the user's"
)

var newBlocks = []newBlock{
	// 1: nested shells, eval, here-strings and heredocs fed to a shell.
	{1, "bash -c 'git push'", pushReason},
	{1, `sh -c "git commit -m x"`, commitReason},
	{1, "zsh -c 'git push'", pushReason},
	{1, "dash -c 'git stash'", stashReason},
	{1, "bash -lc 'git push origin main'", pushReason},
	{1, "sh -ec 'cd app && git push'", pushReason},
	{1, "bash -o pipefail -c 'git tag v1'", tagReason},
	{1, "/bin/bash --norc -c 'dart format lib'", formatReason},
	{1, `bash -c 'gh pr comment 3 --body "/approve plan"'`, approveReason},
	{1, `sh -c "sh -c 'git push'"`, pushReason},
	{1, "eval 'git push'", pushReason},
	{1, "eval git push", pushReason},
	{1, `eval "ls; git reset --hard"`, resetReason},
	{1, "sh <<< 'git push'", pushReason},
	{1, `bash <<< "git commit -m x"`, commitReason},
	{1, "cat <<< 'git push' | sh", pushReason},
	{1, "bash <<'EOF'\nsh -c 'git push'\nEOF", pushReason},
	{1, "bash <<EOF\neval 'git stash'\nEOF", stashReason},
	{1, "echo $(sh -c 'git push')", pushReason},

	// 2: keywords and grouping.
	{2, "{ git push; }", pushReason},
	{2, "{ dart format lib; } > out", formatReason},
	{2, "if git push; then echo ok; fi", pushReason},
	{2, "if true; then git push; fi", pushReason},
	{2, "if false; then :; else git commit -m x; fi", commitReason},
	{2, "if a; then :; elif git push; then :; fi", pushReason},
	{2, "while git push; do sleep 1; done", pushReason},
	{2, "until git push; do sleep 1; done", pushReason},
	{2, "for f in a b; do git stash; done", stashReason},
	{2, "! git push", pushReason},
	{2, "time -p git push", pushReason},
	{2, "f() { git push; }", pushReason},

	// 3: wrappers with options, absolute paths, and wrappers the Dart guard does not list.
	{3, "sudo -u root git push", pushReason},
	{3, "sudo -E -- git push", pushReason},
	{3, "sudo -uroot -H git commit -m x", commitReason},
	{3, "/usr/bin/sudo git push", pushReason},
	{3, "sudo --user=root git push", pushReason},
	{3, "env -i git push", pushReason},
	{3, "env -u HOME git push", pushReason},
	{3, "env -i PATH=/usr/bin FOO=1 git push", pushReason},
	{3, "/usr/bin/env git push", pushReason},
	{3, "env - git push", pushReason},
	{3, "env -S 'git push'", pushReason},
	{3, "nice git push", pushReason},
	{3, "nice -n 10 git push", pushReason},
	{3, "nice -5 git commit -m x", commitReason},
	{3, "ionice -c 3 git push", pushReason},
	{3, "timeout 5 git push", pushReason},
	{3, "timeout -s KILL -k 10 5m git push", pushReason},
	{3, "timeout --preserve-status 30 dart format lib", formatReason},
	{3, "exec git push", pushReason},
	{3, "exec -a name git push", pushReason},
	{3, "command -p git push", pushReason},
	{3, "builtin eval 'git push'", pushReason},
	{3, "nohup -- git push", pushReason},
	{3, "xargs git push", pushReason},
	{3, "xargs -0 -n1 git push", pushReason},
	{3, "ls | xargs -I {} git commit -m {}", commitReason},
	{3, "xargs -P 4 dart format", formatReason},
	{3, `find . -exec git push \;`, pushReason},
	{3, "find . -name '*.dart' -execdir dart format {} +", formatReason},
	{3, "find . -ok git stash ';'", stashReason},
	{3, "rtk proxy git push", pushReason},
	{3, "sudo env -i nice -n 5 timeout 9 git push", pushReason},

	// 4: backslash-newline line continuations.
	{4, "git \\\n push", pushReason},
	{4, "git \\\n  commit -m x", commitReason},
	{4, "git -C app \\\n push", pushReason},
	{4, "sudo \\\n git stash", stashReason},
	{4, "dart \\\n format lib", formatReason},
	{4, "FOO=1 \\\n git reset --hard", resetReason},
	{4, `bash -c "git \` + "\n" + ` push"`, pushReason},
	{4, "x=`git \\\n tag v1`", tagReason},
}

// stillAllowed are commands the hardening must keep allowed.
var stillAllowed = []string{
	`echo "git push"`,
	`grep -r 'sh -c' .`,
	`grep -rn "eval 'git push'" docs/`,
	"timeout 5 flutter test",
	"timeout -s KILL 30 dart test",
	"sudo apt list",
	"sudo -u root ls",
	"env FOO=1 make",
	"env -i PATH=/usr/bin ls",
	"nice -n 5 flutter test",
	"bash script.sh",
	"bash -c 'git status'",
	`sh -c 'echo "git push"'`,
	"bash -lc 'flutter test test/a_test.dart'",
	"eval echo git push",
	"cat <<< 'git push'",
	`grep push <<< "git push"`,
	"command -v git",
	"xargs echo git push",
	"ls | xargs -n1 grep push",
	"find . -name '*.dart' -exec grep -l push {} +",
	`find . -exec echo git push \;`,
	"if git diff --quiet; then echo clean; fi",
	"while read line; do echo \"$line\"; done < f",
	"{ echo a; echo b; } > out",
	"! git diff --quiet",
	"time flutter test",
	"exec bash",
	"nohup flutter run &",
	"git \\\n status",
	"echo a \\\n b",
	"rtk proxy git status",
	"cat > notes.md <<'EOF'\nsh -c 'git push'\nEOF",
}

// indirection is gap 5, open by decision: a `$`-expansion as the first word. Blocking it would also block
// legitimate scripts, so these stay allowed.
var indirection = []string{
	"$(echo git) push",
	"git${IFS}push",
	"$x push",
	`"$GIT" push`,
	"${GIT:-git} commit -m x",
}

func TestNewBlocksAreBlockedByGoAndAllowedByDartRules(t *testing.T) {
	p := testPolicy(false, false)
	perGap := map[int]int{}
	seen := map[string]bool{}
	for _, row := range newBlocks {
		if seen[row.command] {
			t.Errorf("duplicate row %q", row.command)
		}
		seen[row.command] = true
		perGap[row.gap]++
		if evaluateShellAsDart(row.command, p).Blocked {
			t.Errorf("gap %d: %q is blocked by the Dart rules already; it is not a new block", row.gap, row.command)
		}
		if decision := decide(t, shellEvent(row.command), p); !decision.Blocked || !strings.Contains(decision.Reason, row.reason) {
			t.Errorf("gap %d: %q: %+v, want blocked with %q", row.gap, row.command, decision, row.reason)
		}
	}
	for gap := 1; gap <= 4; gap++ {
		if perGap[gap] < 5 {
			t.Errorf("gap %d has %d rows, want at least 5", gap, perGap[gap])
		}
	}
}

func TestHardeningKeepsEverydayCommandsAllowed(t *testing.T) {
	p := testPolicy(false, false)
	for _, command := range stillAllowed {
		if decision := decide(t, shellEvent(command), p); decision.Blocked {
			t.Errorf("%q: %+v", command, decision)
		}
	}
}

func TestIndirectionStaysAllowedByDecision(t *testing.T) {
	p := testPolicy(false, false)
	for _, command := range indirection {
		if decision := decide(t, shellEvent(command), p); decision.Blocked {
			t.Errorf("%q: %+v; gap 5 stays open by decision", command, decision)
		}
	}
}

func TestCodexArgvListsAreUnwrapped(t *testing.T) {
	p := testPolicy(false, false)
	for _, argv := range []string{
		`["bash","-ec","git push"]`, `["bash","-xc","git push"]`, `["sh","-o","errexit","-ec","git push"]`, `["/usr/bin/env","-i","git","push"]`,
		`["sudo","-u","root","git","push"]`, `["dash","-c","git stash"]`, `["xargs","-n1","git","push"]`,
	} {
		events := allFromHook(t, `{"tool_name":"shell","tool_input":{"command":`+argv+`}}`)
		if evaluateShellAsDart(events[0].Command, p).Blocked {
			t.Errorf("%s: blocked by the Dart rules already", argv)
		}
		if decision, err := EvaluateAll(events, p); err != nil || !decision.Blocked {
			t.Errorf("%s: %+v, %v", argv, decision, err)
		}
	}
	for _, argv := range []string{`["echo","git","push"]`, `["bash","-c","git status"]`, `["timeout","5","flutter","test"]`} {
		if decision, err := EvaluateAll(allFromHook(t, `{"tool_name":"shell","tool_input":{"command":`+argv+`}}`), p); err != nil || decision.Blocked {
			t.Errorf("%s: %+v, %v", argv, decision, err)
		}
	}
}

func TestNestingDepthIsLimited(t *testing.T) {
	p := testPolicy(false, false)
	script := "git push"
	for i := 0; i < maxNesting; i++ {
		script = "sh -c " + strconv.Quote(script)
	}
	if !decide(t, shellEvent(script), p).Blocked {
		t.Errorf("%d shells deep: allowed", maxNesting)
	}
	for i := 0; i < 4; i++ {
		script = "eval " + strconv.Quote(script)
	}
	if decide(t, shellEvent(script), p).Blocked {
		t.Errorf("past the limit: blocked")
	}
}

// patchBlock is an apply_patch payload the Dart guard allows and the Go guard blocks (gap 6 of the G4 plan).
type patchBlock struct {
	name, patch, role string
	malformed         bool
}

var newPatchBlocks = []patchBlock{
	{"CRLF update header", "*** Begin Patch\r\n*** Update File: lib/a.dart\r\n+" + vendorImport + "\r\n*** End Patch\r\n", "", false},
	{"CRLF add header", "*** Add File: lib/feature/a.dart\r\n+" + vendorImport + "\r\n", "", false},
	{"CRLF write by a read role", "*** Add File: notes.md\r\n+x\r\n", "reviewer", false},
	{"CRLF write outside the plans by a plans role", "*** Add File: lib/x.dart\r\n+x\r\n", "planner", false},
	{"CRLF second header after an LF one", "*** Update File: test/a.dart\n+x\n*** Update File: lib/b.dart\r\n+" + vendorImport + "\r\n", "", false},
	{"move before any header into lib", "*** Move to: lib/a.dart\n+" + vendorImport + "\n", "", true},
	{"move before any header by a read role", "*** Move to: notes.md\n+x\n", "reviewer", true},
	{"move before a header hides the next file", "*** Move to: test/x.dart\n*** Add File: lib/a.dart\n+" + vendorImport + "\n", "", true},
	{"CRLF move before any header", "*** Move to: lib/a.dart\r\n+" + vendorImport + "\r\n", "", true},
}

func patchHookAs(patch, role string) string {
	agentType := ""
	if role != "" {
		agentType = `, "agent_type": ` + jsonString(role)
	}
	return `{"tool_name": "apply_patch", "tool_input": {"command": ` + jsonString(patch) + `}` + agentType + `}`
}

func TestNewPatchBlocks(t *testing.T) {
	p := testPolicy(false, false)
	for _, row := range newPatchBlocks {
		events := allFromHook(t, patchHookAs(row.patch, row.role))
		if decision, err := EvaluateAll(events, p); err != nil || !decision.Blocked {
			t.Errorf("%s: %+v, %v", row.name, decision, err)
		}
		var r *string
		if row.role != "" {
			r = role(row.role)
		}
		if dart, err := patchEvents(row.patch, r, false); err == nil {
			if decision, _ := EvaluateAll(dart, p); decision.Blocked {
				t.Errorf("%s: blocked by the Dart reading already", row.name)
			}
		}
		malformed := false
		for _, event := range events {
			malformed = malformed || event.Malformed != ""
		}
		if malformed != row.malformed {
			t.Errorf("%s: malformed %v, want %v", row.name, malformed, row.malformed)
		}
	}
	events := allFromHook(t, patchHook("*** Move to: lib/a.dart\n+x\n"))
	if decision, err := EvaluateAll(events, p); err != nil || decision.Blocked || len(events) != 1 || events[0].Malformed == "" {
		t.Errorf("a harmless malformed patch: %+v, %+v, %v", events, decision, err)
	}
	if events := allFromHook(t, patchHook("*** Begin Patch\r\n*** Add File: test/a_test.dart\r\n+x\r\n*** End Patch\r\n")); len(events) != 1 || events[0].FilePath != "test/a_test.dart" || events[0].Content != "x" {
		t.Errorf("CRLF patch: %+v", events)
	}
}

// gitRepo makes a folder that holds a `.git` folder and returns it with its git common dir.
func gitRepo(t *testing.T) (dir, common string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common = filepath.Join(root, ".git")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, common
}

func runIn(t *testing.T, dir, policy, stdin string, now time.Time) int {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := Env{
		LookupEnv: func(string) (string, bool) { return "", false },
		Getwd:     func() (string, error) { return dir, nil },
		Now:       func() time.Time { return now },
	}
	return Run([]string{policy}, strings.NewReader(stdin), &stdout, &stderr, env)
}

func writePolicy(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "guard.json")
	if err := os.WriteFile(path, []byte(corpusPolicies["default"]), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func logLines(t *testing.T, common string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(common, "dashcrew", MalformedLogName))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestMalformedPayloadsAreAllowedAndLogged(t *testing.T) {
	dir, common := gitRepo(t)
	policy := writePolicy(t, dir)
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	const secret = "SECRET-PAYLOAD-TEXT"
	for _, c := range []struct{ stdin, line string }{
		{`{"tool_name":"Bash","tool_input":{"command":"git push ` + secret, "2026-11-01T10:00:00Z\tunknown\tinput is not JSON"},
		{"\xff" + secret, "2026-11-01T10:00:00Z\tunknown\tinput is not UTF-8"},
		{`["` + secret + `"]`, "2026-11-01T10:00:00Z\tunknown\tinput is not a JSON object"},
		{`{"tool_name":7,"tool_input":{"command":"git push ` + secret + `"}}`, "2026-11-01T10:00:00Z\tunknown\thook field tool_name has the wrong type"},
		{`{"tool_name":"Write","tool_input":{"file_path":7,"content":"` + secret + `"}}`, "2026-11-01T10:00:00Z\tclaude-code\thook field file_path has the wrong type"},
		{`{"tool_name":"MultiEdit","tool_input":{"file_path":"lib/a.dart","edits":["` + secret + `"]}}`, "2026-11-01T10:00:00Z\tclaude-code\thook field edits has the wrong type"},
		{`{"toolCall":{"name":"run_command","args":{"CommandLine":7}},"workspacePaths":["` + dir + `"]}`, "2026-11-01T10:00:00Z\tantigravity\thook field CommandLine has the wrong type"},
		{patchHook("*** Move to: lib/" + secret + ".dart\n+x\n"), "2026-11-01T10:00:00Z\tcodex\tapply_patch moves a file before naming one"},
	} {
		if code := runIn(t, dir, policy, c.stdin, now); code != 0 {
			t.Errorf("%q: exit %d, want 0", c.stdin, code)
		}
		lines := logLines(t, common)
		if len(lines) == 0 || lines[len(lines)-1] != c.line {
			t.Errorf("%q: log %q, want last line %q", c.stdin, lines, c.line)
		}
	}
	data, _ := os.ReadFile(filepath.Join(common, "dashcrew", MalformedLogName))
	if strings.Contains(string(data), secret) {
		t.Errorf("the log quotes a payload:\n%s", data)
	}

	before := len(logLines(t, common))
	for _, stdin := range []string{
		`{"tool_name":"Bash","tool_input":{"command":"git status"}}`,
		patchHook("*** Update File: lib/a.dart\n*** Move to: lib/b.dart\n+x\n"),
		`{"tool_name":"WebFetch","tool_input":{}}`,
	} {
		runIn(t, dir, policy, stdin, now)
	}
	if after := len(logLines(t, common)); after != before {
		t.Errorf("well-formed payloads logged %d lines", after-before)
	}
	if code := runIn(t, dir, policy, patchHook("*** Move to: lib/a.dart\n+"+vendorImport+"\n"), now); code != 2 {
		t.Errorf("malformed patch with a forbidden import: exit %d, want 2", code)
	}
}

func TestMalformedLogFollowsTheHookCwdAndWorktrees(t *testing.T) {
	main, common := gitRepo(t)
	worktreeGit := filepath.Join(common, "worktrees", "wt")
	if err := os.MkdirAll(worktreeGit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGit, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(main), "wt")
	if err := os.MkdirAll(filepath.Join(worktree, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+worktreeGit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := GitCommonDir(filepath.Join(worktree, "lib")); !ok || got != common {
		t.Fatalf("GitCommonDir of the worktree: %q, %v; want %q", got, ok, common)
	}
	elsewhere := t.TempDir()
	policy := writePolicy(t, elsewhere)
	runIn(t, elsewhere, policy, `{"tool_name":"Write","tool_input":{"file_path":7},"cwd":"`+filepath.Join(worktree, "lib")+`"}`, time.Now())
	if lines := logLines(t, common); len(lines) != 1 || !strings.HasSuffix(lines[0], "\tclaude-code\thook field file_path has the wrong type") {
		t.Errorf("log %q", lines)
	}
	if _, ok := GitCommonDir(elsewhere); ok {
		t.Skip("the temp folder is inside a git repository")
	}
	if code := runIn(t, elsewhere, policy, "{", time.Now()); code != 0 {
		t.Errorf("outside a repository: exit %d", code)
	}
}

func TestMalformedLogIsCapped(t *testing.T) {
	dir, common := gitRepo(t)
	policy := writePolicy(t, dir)
	path := filepath.Join(common, "dashcrew", MalformedLogName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	full := strings.Repeat("2026-01-01T00:00:00Z\tunknown\tinput is not JSON\n", MalformedLogLimit/47)
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	runIn(t, dir, policy, "{", time.Now())
	if info, err := os.Stat(path); err != nil || info.Size() > MalformedLogLimit {
		t.Errorf("log size %v, %v; limit %d", info.Size(), err, MalformedLogLimit)
	}
}

func TestMalformedSinceSyncCountsLinesAfterTheLastSync(t *testing.T) {
	dir, common := gitRepo(t)
	if count, path := MalformedSinceSync(dir); count != 0 || path != filepath.Join(common, "dashcrew", MalformedLogName) {
		t.Errorf("no log: %d, %q", count, path)
	}
	policy := writePolicy(t, dir)
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	runIn(t, dir, policy, "{", day)
	runIn(t, dir, policy, "[]", day.Add(time.Hour))
	if count, _ := MalformedSinceSync(dir); count != 2 {
		t.Errorf("before any sync: %d, want 2", count)
	}
	if err := MarkSynced(dir, day.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if count, _ := MalformedSinceSync(dir); count != 0 {
		t.Errorf("right after sync: %d, want 0", count)
	}
	runIn(t, dir, policy, "{", day.Add(3*time.Hour))
	if count, _ := MalformedSinceSync(dir); count != 1 {
		t.Errorf("after sync: %d, want 1", count)
	}
	outside := t.TempDir()
	if _, ok := GitCommonDir(outside); !ok {
		if err := MarkSynced(outside, day); err != nil {
			t.Errorf("MarkSynced outside a repository: %v", err)
		}
	}
}

// corpusNewBlock is a line of testdata/corpus_new_blocks.txt: a differential corpus case that the Go guard blocks
// and the Dart guard allows, keyed by its shell script (or its whole payload when it has none).
type corpusNewBlock struct {
	policy, reason, key string
}

const corpusNewBlocksPath = "testdata/corpus_new_blocks.txt"

func (b corpusNewBlock) String() string {
	return b.policy + "\t" + b.reason + "\t" + jsonString(b.key)
}

func readCorpusNewBlocks(t testing.TB) []corpusNewBlock {
	t.Helper()
	file, err := os.Open(corpusNewBlocksPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var out []corpusNewBlock
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	for lines.Scan() {
		if lines.Text() == "" || strings.HasPrefix(lines.Text(), "#") {
			continue
		}
		fields := strings.SplitN(lines.Text(), "\t", 3)
		if len(fields) != 3 {
			t.Fatalf("%s: bad line %q", corpusNewBlocksPath, lines.Text())
		}
		value, err := dartjson.Decode(fields[2])
		key, ok := value.(string)
		if err != nil || !ok {
			t.Fatalf("%s: bad key in %q", corpusNewBlocksPath, lines.Text())
		}
		out = append(out, corpusNewBlock{fields[0], fields[1], key})
	}
	return out
}

// newBlockKey is what identifies a corpus case in the new-blocks tables: the shell script of a shell call, else
// the whole payload.
func newBlockKey(stdin string) string {
	if object := hookObject(stdin); object != nil {
		if events, err := AllFromHook(object); err == nil && len(events) == 1 && events[0].Kind == Shell {
			return events[0].Command
		}
	}
	return stdin
}

// TestCorpusNewBlocksAreBlockedWithTheirReason checks every recorded line in process: the guard must keep
// blocking each one with its reason.
func TestCorpusNewBlocksAreBlockedWithTheirReason(t *testing.T) {
	f := newCorpusFolders(t)
	recorded := readCorpusNewBlocks(t)
	if len(recorded) == 0 {
		t.Fatal("no recorded new blocks")
	}
	seen := map[string]bool{}
	for _, b := range recorded {
		if seen[b.String()] {
			t.Errorf("duplicate line %s", b)
		}
		seen[b.String()] = true
		stdin := b.key
		if hookObject(stdin) == nil {
			stdin = `{"tool_name":"Bash","tool_input":{"command":` + jsonString(b.key) + `}}`
		}
		var stdout, stderr bytes.Buffer
		env := Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return f.elsewhere, nil }}
		Run([]string{filepath.Join(f.policies, b.policy+".json")}, strings.NewReader(stdin), &stdout, &stderr, env)
		if got := strings.TrimSpace(strings.TrimPrefix(stderr.String(), "[dashcrew guard] ")); got != b.reason {
			t.Errorf("%s\n  got reason %q", b, got)
		}
	}
	t.Logf("%d recorded corpus new blocks", len(recorded))
}

// FuzzEvaluateShell checks that the Go shell rules never panic, are deterministic, and block every script the
// Dart rules block, with the same reason.
func FuzzEvaluateShell(f *testing.F) {
	for _, seed := range append(seedCommands(), hardeningSeeds()...) {
		f.Add(seed)
	}
	p := testPolicy(false, false)
	f.Fuzz(func(t *testing.T, script string) {
		dart := evaluateShellAsDart(script, p)
		got := evaluateShell(script, nil, p)
		if dart.Blocked && got != dart {
			t.Fatalf("%q: Dart rules %+v, Go %+v", script, dart, got)
		}
		if again := evaluateShell(script, nil, p); again != got {
			t.Fatalf("%q: not deterministic: %+v then %+v", script, got, again)
		}
	})
}
