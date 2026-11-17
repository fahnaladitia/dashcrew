package guard

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// runUnknown runs the guard on stdin in a git repository with the default policy and returns the exit code, stdout
// and the last malformed log line ("" when nothing was logged).
func runUnknown(t *testing.T, stdin string) (int, string, string) {
	t.Helper()
	dir, common := gitRepo(t)
	policy := writePolicy(t, dir)
	var stdout, stderr bytes.Buffer
	env := Env{
		LookupEnv: func(string) (string, bool) { return "", false },
		Getwd:     func() (string, error) { return dir, nil },
		Now:       func() time.Time { return time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC) },
	}
	code := Run([]string{policy}, strings.NewReader(stdin), &stdout, &stderr, env)
	lines := logLines(t, common)
	if len(lines) == 0 {
		return code, stdout.String(), ""
	}
	return code, stdout.String(), lines[len(lines)-1]
}

// TestAnUnknownToolIsCheckedOrLogged pins that no input the guard does not recognize passes in silence: a command
// under any tool name is checked as a shell command, and every unknown tool is logged for doctor guard-log.
func TestAnUnknownToolIsCheckedOrLogged(t *testing.T) {
	long := strings.Repeat("x", 65)
	for _, c := range []struct {
		name, stdin string
		blocked     bool
		log         string
	}{
		{"unknown tool with a blocked command", `{"tool_name":"RunTerminal","tool_input":{"command":"git push"}}`, true,
			"claude-code\tunknown tool RunTerminal, checked as a shell command"},
		{"unknown tool with an allowed command", `{"tool_name":"RunTerminal","tool_input":{"command":"ls"}}`, false,
			"claude-code\tunknown tool RunTerminal, checked as a shell command"},
		{"unknown tool with an argv command", `{"tool_name":"run","tool_input":{"command":["bash","-c","git push"]}}`, true,
			"claude-code\tunknown tool run, checked as a shell command"},
		{"no tool name, top-level command", `{"hook_event_name":"beforeRun","command":"git push"}`, true,
			"unknown\tinput without a tool name, checked as a shell command"},
		{"unknown tool with an empty command", `{"tool_name":"RunTerminal","tool_input":{"command":""}}`, false,
			"claude-code\tunknown tool RunTerminal, allowed"},
		{"read is not guessed into a write", `{"tool_name":"Read","tool_input":{"file_path":"lib/a.dart"},"agent_type":"reviewer"}`, false,
			"claude-code\tunknown tool Read, allowed"},
		{"Cursor delete", `{"hook_event_name":"preToolUse","cursor_version":"3.1.0","tool_name":"Delete","tool_input":{"file_path":".env"}}`, false,
			"cursor\tunknown tool Delete, allowed"},
		{"a name with spaces is not shown", `{"tool_name":"run it now","tool_input":{}}`, false,
			"claude-code\tunknown tool (name not shown), allowed"},
		{"a long name is not shown", `{"tool_name":"` + long + `","tool_input":{}}`, false,
			"claude-code\tunknown tool (name not shown), allowed"},
		{"known tool is not logged", `{"tool_name":"Bash","tool_input":{"command":"ls"}}`, false, ""},
		{"known Cursor shell event is not logged", `{"hook_event_name":"beforeShellExecution","cursor_version":"3.1.0","command":"ls"}`, false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, _, line := runUnknown(t, c.stdin)
			if blocked := code == 2; blocked != c.blocked {
				t.Errorf("exit %d, want blocked %v", code, c.blocked)
			}
			switch {
			case c.log == "" && line != "":
				t.Errorf("logged %q, want nothing", line)
			case c.log != "" && !strings.HasSuffix(line, "\t"+c.log):
				t.Errorf("log %q, want a line ending in %q", line, c.log)
			}
		})
	}
}

func TestAnUnknownAntigravityToolWithACommandIsDenied(t *testing.T) {
	code, stdout, line := runUnknown(t, `{"toolCall":{"name":"run_terminal","args":{"CommandLine":"git push"}}}`)

	if code != 0 || !strings.Contains(stdout, `"decision":"deny"`) {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	if want := "antigravity\tunknown tool run_terminal, checked as a shell command"; !strings.HasSuffix(line, "\t"+want) {
		t.Errorf("log %q, want %q", line, want)
	}
}

func TestAnUnknownAntigravityToolWithoutACommandIsAllowedAndLogged(t *testing.T) {
	code, stdout, line := runUnknown(t, `{"toolCall":{"name":"view_file","args":{"AbsolutePath":"/repo/a.dart"}}}`)

	if code != 0 || strings.TrimSpace(stdout) != `{"decision":"allow"}` {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	if want := "antigravity\tunknown tool view_file, allowed"; !strings.HasSuffix(line, "\t"+want) {
		t.Errorf("log %q, want %q", line, want)
	}
}
