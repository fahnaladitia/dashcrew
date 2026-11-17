package guard

import (
	"bytes"
	"strings"
	"testing"
)

// cursorShellInputs are the two documented Cursor inputs of a shell command (https://cursor.com/docs/hooks):
// `preToolUse` for the `Shell` tool and `beforeShellExecution`.
func cursorShellInputs(command string) map[string]string {
	return map[string]string{
		"preToolUse Shell": `{"hook_event_name":"preToolUse","cursor_version":"3.1.0","tool_name":"Shell","tool_input":{"command":` +
			jsonString(command) + `,"working_directory":"/repo"},"cwd":"/repo"}`,
		"beforeShellExecution": `{"hook_event_name":"beforeShellExecution","cursor_version":"3.1.0","command":` +
			jsonString(command) + `,"cwd":"/repo","sandbox":false}`,
	}
}

// TestCursorShellInputsAreDecidedAsClaudeCodeBash pins that a command Cursor sends gets the answer the same command
// gets from Claude Code; before this, Cursor's `Shell` tool and `beforeShellExecution` were allowed unread.
func TestCursorShellInputsAreDecidedAsClaudeCodeBash(t *testing.T) {
	var commands []string
	for _, entry := range append(append([][2]string{}, blockedShell...), allowedShell...) {
		commands = append(commands, entry[0])
	}
	for _, command := range commands {
		want := blockedAll(t, allFromHook(t, `{"tool_name":"Bash","tool_input":{"command":`+jsonString(command)+`}}`))
		for name, input := range cursorShellInputs(command) {
			if got := blockedAll(t, allFromHook(t, input)); got != want {
				t.Errorf("%s %q: blocked %v, Claude Code Bash %v", name, command, got, want)
			}
		}
	}
}

// TestRunBlocksCursorShellWithExitTwo pins the answer Cursor documents as a block for both its own hooks and the
// Claude Code hooks it runs: exit 2, with the reason on stderr.
func TestRunBlocksCursorShellWithExitTwo(t *testing.T) {
	dir := t.TempDir()
	policy := writePolicy(t, dir)
	for name, input := range cursorShellInputs("bash -c 'git push'") {
		var stdout, stderr bytes.Buffer

		code := Run([]string{policy}, strings.NewReader(input), &stdout, &stderr, Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return dir, nil }})

		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "[dashcrew guard] git push") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout.String(), stderr.String())
		}
	}
}

func TestCursorIsNamedInTheMalformedLog(t *testing.T) {
	for name, input := range cursorShellInputs("ls") {
		if got := agentOf(hook(t, input)); got != "cursor" {
			t.Errorf("%s: agent %q", name, got)
		}
	}
}
