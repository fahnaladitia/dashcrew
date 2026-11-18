package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

const askProfile = "toolkit: 1\nagents: [claude-code]\npolicy: { agent_commit: ask, agent_push: ask }\n"

func TestAskPolicyRoundTripsAndRaisesTheMinimum(t *testing.T) {
	p := compiled(t, askProfile)
	if !p.AskGit["commit"] || !p.AskGit["push"] || p.BlockedGit["commit"] || p.BlockedGit["push"] || p.Push == nil {
		t.Fatalf("compiled policy: ask %v, blocked %v, push %v", p.AskGit, p.BlockedGit, p.Push)
	}
	text, _ := dartjson.EncodeIndent(p.ToJSON(), "  ")
	if !strings.Contains(text, `"ask_git": [`) {
		t.Errorf("guard.json lacks ask_git:\n%s", text)
	}
	read, err := PolicyFromJSON(text)
	if err != nil || !read.AskGit["commit"] || !read.AskGit["push"] {
		t.Errorf("read back: %v %v", read.AskGit, err)
	}
	if p.Minimum() != version.AskGuardMinimum {
		t.Errorf("Minimum = %s, want %s", p.Minimum(), version.AskGuardMinimum)
	}
	if plain := compiled(t, goOther); strings.Contains(mustJSON(t, plain), "ask_git") {
		t.Error("a policy without ask writes ask_git")
	}
}

func mustJSON(t *testing.T, p Policy) string {
	t.Helper()
	text, err := dartjson.EncodeIndent(p.ToJSON(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestAskDecisions(t *testing.T) {
	p := compiled(t, askProfile)
	cases := []struct {
		command string
		want    string // allow, ask or block
	}{
		{"git commit -m x", "ask"},
		{`bash -c "git commit -m x"`, "ask"},
		{"env FOO=1 git push origin feat/x", "ask"},
		{"git -C . push -u origin feat/x", "ask"},
		{"git status && git commit -m x", "ask"},
		{"git push --force origin feat/x", "block"},
		{"git push origin main", "block"},
		{"git reset --hard", "block"},
		{"git commit -m x && git rebase main", "block"},
		{"git status", "allow"},
	}
	for _, c := range cases {
		decision, err := EvaluateAll([]Event{shellEvent(c.command)}, p)
		if err != nil {
			t.Fatal(err)
		}
		got := "allow"
		switch {
		case decision.Blocked:
			got = "block"
		case decision.Ask:
			got = "ask"
		}
		if got != c.want {
			t.Errorf("%q: %s (%s), want %s", c.command, got, decision.Reason, c.want)
		}
	}
	decision, _ := EvaluateAll([]Event{shellEvent("git commit -m x")}, p)
	if decision.Reason != "git commit needs your approval (policy.agent_commit: ask)" {
		t.Errorf("reason %q", decision.Reason)
	}
}

func TestABlockInAnotherEventWinsOverAnAsk(t *testing.T) {
	p := compiled(t, askProfile)
	decision, _ := EvaluateAll([]Event{shellEvent("git commit -m x"), shellEvent("git reset --hard")}, p)
	if !decision.Blocked {
		t.Errorf("got %+v, want the reset blocked", decision)
	}
}

func TestAskIsAnsweredOnlyToAnAgentThatHonoursIt(t *testing.T) {
	ask := Decision{Ask: true, Reason: "git commit needs your approval (policy.agent_commit: ask)"}
	claude := hook(t, `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit"}}`)
	agy := hook(t, `{"toolCall":{"name":"run_command","args":{"CommandLine":"git commit"}}}`)

	got := HookResponse(claude, ask, "claude-code")
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"[dashcrew guard] git commit needs your approval (policy.agent_commit: ask)"}}`
	if got.ExitCode != 0 || got.Stdout != want || got.Stderr != "" {
		t.Errorf("Claude Code: %+v", got)
	}

	got = HookResponse(agy, ask, "")
	if got.ExitCode != 0 || got.Stdout != `{"decision":"force_ask","reason":"[dashcrew guard] git commit needs your approval (policy.agent_commit: ask)"}` {
		t.Errorf("Antigravity: %+v", got)
	}

	// The same Bash payload without the Claude Code argument may be Codex, which runs a call its hook asked about.
	got = HookResponse(claude, ask, "")
	if got.ExitCode != 2 || got.Stdout != "" || !strings.Contains(got.Stderr, "cannot ask for approval") {
		t.Errorf("unknown agent: %+v", got)
	}
}

func TestRunPassesTheAgentArgument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guard.json")
	if err := os.WriteFile(path, []byte(mustJSON(t, compiled(t, askProfile))), 0o644); err != nil {
		t.Fatal(err)
	}
	env := Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return dir, nil }}
	input := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m x"},"cwd":"` + dir + `"}`

	var out, errOut strings.Builder
	code := Run([]string{path, "claude-code"}, strings.NewReader(input), &out, &errOut, env)
	if code != 0 || !strings.Contains(out.String(), `"permissionDecision":"ask"`) {
		t.Errorf("with the argument: exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{path}, strings.NewReader(input), &out, &errOut, env); code != 2 {
		t.Errorf("without the argument: exit %d, stdout %q", code, out.String())
	}
}
