package guard

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// dartGuardTests is the number of tests in test/unit/guard_test.dart. Every one of them is a row of guardCases,
// named after the Dart test.
const dartGuardTests = 58

func testPolicy(agentCommit, formatter bool) Policy {
	blocked := map[string]bool{}
	for _, sub := range AlwaysBlockedGit {
		blocked[sub] = true
	}
	if !agentCommit {
		blocked["commit"] = true
	}
	return Policy{
		BlockedGit:       blocked,
		BlockFormatters:  !formatter,
		PlansDir:         ".ai/plans",
		RoleCapability:   map[string]string{"planner": "write-plans", "reviewer": "read", "engineer": "write-code"},
		ForbiddenImports: []string{"package:vendor_sdk/"},
		LocalPackages:    []string{"core"},
	}
}

func role(name string) *string { return &name }

func shellEvent(command string) Event { return Event{Kind: Shell, Command: command} }

func writeEvent(path string, r *string, kind Kind) Event {
	return Event{Kind: kind, FilePath: path, Content: "x", Role: r}
}

func hook(t *testing.T, text string) *omap.Map[any] {
	t.Helper()
	value, err := dartjson.Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	return value.(*omap.Map[any])
}

func blocked(t *testing.T, event Event, p Policy) bool {
	t.Helper()
	return decide(t, event, p).Blocked
}

func decide(t *testing.T, event Event, p Policy) Decision {
	t.Helper()
	decision, err := Evaluate(event, p)
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

func allFromHook(t *testing.T, text string) []Event {
	t.Helper()
	events, err := AllFromHook(hook(t, text))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func blockedAll(t *testing.T, events []Event) bool {
	t.Helper()
	decision, err := EvaluateAll(events, testPolicy(false, false))
	if err != nil {
		t.Fatal(err)
	}
	return decision.Blocked
}

func jsonString(s string) string {
	text, _ := dartjson.Encode(s)
	return text
}

type guardCase struct {
	dart string
	run  func(t *testing.T)
}

var blockedShell = [][2]string{
	{"git commit -m x", "commit"},
	{"git push", "push"},
	{"rtk git push origin main", "rtk wrapper"},
	{"sudo git stash", "sudo wrapper"},
	{"FOO=1 git reset --hard", "env assignment"},
	{"git -C app commit -m x", "global -C option"},
	{"git -c user.name=a commit", "global -c option"},
	{"git --no-pager rebase main", "global --no-pager"},
	{"ls && git merge dev", "after &&"},
	{`echo "$(git push)"`, "substitution in quotes"},
	{"dart format lib/", "formatter"},
	{"dart fix --apply", "dart fix"},
	{"flutter format lib/", "flutter format"},
	{"git tag v1", "tag"},
	{"git cherry-pick abc", "cherry-pick"},
	{"git revert abc", "revert"},
	{"bash <<EOF\ngit push\nEOF", "heredoc run by bash"},
	{"ssh host <<'EOF'\ngit push\nEOF", "heredoc run over ssh"},
	{"cat <<EOF | sh\ngit commit -m x\nEOF", "heredoc piped into sh"},
}

var allowedShell = [][2]string{
	{"git status", "read-only git"},
	{"git diff --stat", "diff"},
	{"git log --oneline dev..HEAD", "log"},
	{`grep -rn "git commit" docs/`, "quoted text"},
	{"cat > a.md <<'EOF'\nnever git push here\nEOF", "heredoc body"},
	{"python3 - <<'PY'\nprint('git push')\nPY", "heredoc read by python"},
	{"dart analyze lib/", "analyze"},
	{"flutter test test/a_test.dart", "test"},
	{"git checkout --no-track -b feat/x main", "branch creation"},
	{"echo commit", "word commit alone"},
}

const vendorImport = "import 'package:vendor_sdk/vendor_sdk.dart';"

func patchHook(patch string) string {
	return `{"tool_name": "apply_patch", "tool_input": {"command": ` + jsonString(patch) + `}}`
}

func antigravityHook(name, args string) string {
	return `{"conversationId": "c1", "workspacePaths": ["/repo"], "toolCall": {"name": "` + name + `", "args": ` + args + `}}`
}

func guardCases() []guardCase {
	var cases []guardCase
	for _, entry := range blockedShell {
		command, why := entry[0], entry[1]
		cases = append(cases, guardCase{fmt.Sprintf("blocks: %s (%s)", why, command), func(t *testing.T) {
			if !blocked(t, shellEvent(command), testPolicy(false, false)) {
				t.Errorf("allowed %q", command)
			}
		}})
	}
	for _, entry := range allowedShell {
		command, why := entry[0], entry[1]
		cases = append(cases, guardCase{fmt.Sprintf("allows: %s (%s)", why, command), func(t *testing.T) {
			if blocked(t, shellEvent(command), testPolicy(false, false)) {
				t.Errorf("blocked %q", command)
			}
		}})
	}

	cases = append(cases,
		guardCase{"agent_commit true allows commit but still blocks push", func(t *testing.T) {
			p := testPolicy(true, false)
			if blocked(t, shellEvent("git commit -m x"), p) {
				t.Error("commit blocked")
			}
			if !blocked(t, shellEvent("git push"), p) {
				t.Error("push allowed")
			}
		}},
		guardCase{"formatter true allows dart format", func(t *testing.T) {
			if blocked(t, shellEvent("dart format lib/"), testPolicy(false, true)) {
				t.Error("dart format blocked")
			}
		}},
		guardCase{"block reason names the command", func(t *testing.T) {
			if reason := decide(t, shellEvent("git -C app push"), testPolicy(false, false)).Reason; !strings.Contains(reason, "git push") {
				t.Errorf("reason %q", reason)
			}
		}},
		guardCase{"write-plans role may write under the plans dir only", func(t *testing.T) {
			p := testPolicy(false, false)
			if blocked(t, writeEvent("/repo/.ai/plans/orders/plan.md", role("planner"), Write), p) {
				t.Error("plan write blocked")
			}
			if !blocked(t, writeEvent("/repo/lib/main.dart", role("planner"), Write), p) {
				t.Error("source write allowed")
			}
			if !blocked(t, writeEvent("/repo/.ai/plans/orders/plan.md", role("planner"), Edit), p) {
				t.Error("plan edit allowed")
			}
		}},
		guardCase{"read role cannot write or edit", func(t *testing.T) {
			p := testPolicy(false, false)
			if !blocked(t, writeEvent("/repo/.ai/plans/x.md", role("reviewer"), Write), p) {
				t.Error("write allowed")
			}
			if !blocked(t, writeEvent("/repo/lib/a.dart", role("reviewer"), Edit), p) {
				t.Error("edit allowed")
			}
		}},
		guardCase{"write-code role and the main session write freely", func(t *testing.T) {
			p := testPolicy(false, false)
			if blocked(t, writeEvent("/repo/lib/a.dart", role("engineer"), Write), p) || blocked(t, writeEvent("/repo/lib/a.dart", nil, Write), p) {
				t.Error("blocked")
			}
		}},
		guardCase{"unknown role is not restricted", func(t *testing.T) {
			if blocked(t, writeEvent("/repo/lib/a.dart", role("Explore"), Write), testPolicy(false, false)) {
				t.Error("blocked")
			}
		}},
		guardCase{"forbidden import is blocked in app lib only", func(t *testing.T) {
			p := testPolicy(false, false)
			content := vendorImport + "\n"
			event := func(path string) Event { return Event{Kind: Write, FilePath: path, Content: content} }
			if !blocked(t, event("/repo/lib/feature/a.dart"), p) {
				t.Error("app lib allowed")
			}
			if blocked(t, event("/repo/core/lib/a.dart"), p) {
				t.Error("local package blocked")
			}
			if blocked(t, event("/repo/test/a_test.dart"), p) {
				t.Error("test blocked")
			}
		}},

		guardCase{"GuardEvent.fromHook claude code bash and write input", func(t *testing.T) {
			bash, err := FromHook(hook(t, `{"tool_name": "Bash", "tool_input": {"command": "git push"}, "agent_type": "planner"}`))
			if err != nil || bash.Kind != Shell || bash.Command != "git push" || bash.Role == nil || *bash.Role != "planner" {
				t.Errorf("bash %+v, %v", bash, err)
			}
			edit, err := FromHook(hook(t, `{"tool_name": "Edit", "tool_input": {"file_path": "/repo/lib/a.dart", "new_string": "y"}}`))
			if err != nil || edit.Kind != Edit || edit.FilePath != "/repo/lib/a.dart" || edit.Content != "y" {
				t.Errorf("edit %+v, %v", edit, err)
			}
		}},
		guardCase{"GuardEvent.fromHook codex shell input as argv list unwraps bash -lc", func(t *testing.T) {
			event, err := FromHook(hook(t, `{"tool_name": "shell", "tool_input": {"command": ["bash", "-lc", "git push origin main"]}}`))
			if err != nil || event.Kind != Shell || event.Command != "git push origin main" {
				t.Errorf("%+v, %v", event, err)
			}
		}},
		guardCase{"GuardEvent.fromHook unknown tool is ignored", func(t *testing.T) {
			event, err := FromHook(hook(t, `{"tool_name": "WebFetch", "tool_input": {}}`))
			if err != nil || event.Kind != Other {
				t.Errorf("%+v, %v", event, err)
			}
		}},

		guardCase{"codex apply_patch one event per file, with added lines as content", func(t *testing.T) {
			events := allFromHook(t, patchHook("*** Begin Patch\n*** Add File: test/a_test.dart\n+void main() {}\n*** Update File: lib/b.dart\n@@\n-old()\n+fresh()\n*** Delete File: lib/c.dart\n*** End Patch\n"))
			var paths []string
			var kinds []Kind
			for _, e := range events {
				paths = append(paths, e.FilePath)
				kinds = append(kinds, e.Kind)
			}
			if !reflect.DeepEqual(paths, []string{"test/a_test.dart", "lib/b.dart", "lib/c.dart"}) {
				t.Errorf("paths %q", paths)
			}
			if !reflect.DeepEqual(kinds, []Kind{Write, Edit, Edit}) {
				t.Errorf("kinds %v", kinds)
			}
			if events[1].Content != "fresh()" {
				t.Errorf("content %q", events[1].Content)
			}
		}},
		guardCase{"codex apply_patch a forbidden import in the second file blocks the patch", func(t *testing.T) {
			if !blockedAll(t, allFromHook(t, patchHook("*** Begin Patch\n*** Update File: test/a_test.dart\n+"+vendorImport+"\n*** Update File: lib/feature/b.dart\n+"+vendorImport+"\n*** End Patch\n"))) {
				t.Error("allowed")
			}
		}},
		guardCase{"codex apply_patch removing a forbidden import is allowed", func(t *testing.T) {
			if blockedAll(t, allFromHook(t, patchHook("*** Begin Patch\n*** Update File: lib/feature/b.dart\n-"+vendorImport+"\n+import 'package:app/analytics.dart';\n*** End Patch\n"))) {
				t.Error("blocked")
			}
		}},
		guardCase{"codex apply_patch a move into lib is checked at its new path", func(t *testing.T) {
			if !blockedAll(t, allFromHook(t, patchHook("*** Begin Patch\n*** Update File: test/x.dart\n*** Move to: lib/x.dart\n+"+vendorImport+"\n*** End Patch\n"))) {
				t.Error("allowed")
			}
		}},
		guardCase{"codex apply_patch codex Bash input is one shell event", func(t *testing.T) {
			events := allFromHook(t, `{"tool_name": "Bash", "tool_input": {"command": "git push"}}`)
			if len(events) != 1 || events[0].Kind != Shell || !blockedAll(t, events) {
				t.Errorf("%+v", events)
			}
		}},

		guardCase{"antigravity toolCall input run_command is a shell event", func(t *testing.T) {
			events := allFromHook(t, antigravityHook("run_command", `{"CommandLine": "git push", "Cwd": "/repo"}`))
			if len(events) != 1 || events[0].Kind != Shell {
				t.Errorf("%+v", events)
			}
			if !blockedAll(t, allFromHook(t, antigravityHook("run_command", `{"CommandLine": "git push"}`))) {
				t.Error("push allowed")
			}
			if blockedAll(t, allFromHook(t, antigravityHook("run_command", `{"CommandLine": "git status"}`))) {
				t.Error("status blocked")
			}
		}},
		guardCase{"antigravity toolCall input write_to_file and replace_file_content carry the path and new text", func(t *testing.T) {
			imp := jsonString(vendorImport)
			if !blockedAll(t, allFromHook(t, antigravityHook("write_to_file", `{"TargetFile": "lib/feature/a.dart", "CodeContent": `+imp+`}`))) {
				t.Error("write allowed")
			}
			if !blockedAll(t, allFromHook(t, antigravityHook("replace_file_content", `{"TargetFile": "lib/feature/a.dart", "TargetContent": "x", "ReplacementContent": `+imp+`}`))) {
				t.Error("replace allowed")
			}
			if blockedAll(t, allFromHook(t, antigravityHook("replace_file_content", `{"TargetFile": "lib/feature/a.dart", "TargetContent": `+imp+`, "ReplacementContent": ""}`))) {
				t.Error("removal blocked")
			}
			if events := allFromHook(t, antigravityHook("write_to_file", `{"TargetFile": "a.dart", "CodeContent": ""}`)); len(events) != 1 || events[0].Kind != Write {
				t.Errorf("%+v", events)
			}
		}},
		guardCase{"antigravity toolCall input multi_replace_file_content checks only replacement text of every chunk", func(t *testing.T) {
			imp := jsonString(vendorImport)
			if !blockedAll(t, allFromHook(t, antigravityHook("multi_replace_file_content", `{"TargetFile": "lib/feature/a.dart", "ReplacementChunks": [{"TargetContent": "a", "ReplacementContent": "b"}, {"TargetContent": "c", "ReplacementContent": `+imp+`}]}`))) {
				t.Error("allowed")
			}
			if blockedAll(t, allFromHook(t, antigravityHook("multi_replace_file_content", `{"TargetFile": "lib/feature/a.dart", "ReplacementChunks": [{"TargetContent": `+imp+`, "ReplacementContent": ""}]}`))) {
				t.Error("blocked")
			}
		}},
		guardCase{"antigravity toolCall input other tools and missing args are allowed", func(t *testing.T) {
			if events := allFromHook(t, antigravityHook("view_file", `{"AbsolutePath": "/repo/a"}`)); len(events) != 1 || events[0].Kind != Other {
				t.Errorf("%+v", events)
			}
			if blockedAll(t, allFromHook(t, antigravityHook("run_command", `{}`))) {
				t.Error("blocked")
			}
		}},

		guardCase{"hookResponse claude code and codex get exit 2 and the reason on stderr", func(t *testing.T) {
			response := HookResponse(hook(t, `{"tool_name": "Bash"}`), Block("git push is reserved for the user. Ask them to run it."))
			want := Response{ExitCode: 2, Stderr: "[dashcrew guard] git push is reserved for the user. Ask them to run it."}
			if response != want {
				t.Errorf("%+v", response)
			}
		}},
		guardCase{"hookResponse antigravity gets a deny decision on stdout and exit 0", func(t *testing.T) {
			response := HookResponse(hook(t, `{"toolCall": {"name": "run_command"}}`), Block("git push is reserved for the user. Ask them to run it."))
			want := Response{Stdout: `{"decision":"deny","reason":"[dashcrew guard] git push is reserved for the user. Ask them to run it."}`}
			if response != want {
				t.Errorf("%+v", response)
			}
		}},
		guardCase{"hookResponse an allowed call prints nothing for every agent", func(t *testing.T) {
			for _, input := range []string{`{"tool_name": "Bash"}`, `{"toolCall": {"name": "run_command"}}`} {
				if response := HookResponse(hook(t, input), Allow()); response != (Response{}) {
					t.Errorf("%s: %+v", input, response)
				}
			}
		}},

		guardCase{"an older guard.json that still carries plan_gate loads, and the key is dropped", func(t *testing.T) {
			object := testPolicy(false, false).ToJSON()
			object.Set("plan_gate", true)
			loaded, err := LoadPolicy(object)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.ToJSON().Has("plan_gate") {
				t.Error("plan_gate kept")
			}
			if blocked(t, Event{Kind: Write, FilePath: "/repo/lib/a.dart", Content: "x"}, loaded) {
				t.Error("write blocked")
			}
			if !blocked(t, shellEvent(`gh issue comment 42 --body "/approve plan"`), loaded) {
				t.Error("approve allowed")
			}
		}},
	)

	for _, command := range []string{
		`gh issue comment 42 --body "/approve plan"`,
		`gh api repos/a/b/issues/42/comments -f body='/approve plan'`,
		`rtk gh pr comment 3 --body "/approve plan"`,
	} {
		cases = append(cases, guardCase{"approvals are the user's blocks " + command, func(t *testing.T) {
			decision := decide(t, shellEvent(command), testPolicy(false, false))
			if !decision.Blocked || !strings.Contains(decision.Reason, "Approvals are the user's") || strings.Contains(decision.Reason, "dashcrew gate") {
				t.Errorf("%+v", decision)
			}
		}})
	}
	cases = append(cases,
		guardCase{"approvals are the user's reading issues and mentioning approve elsewhere is fine", func(t *testing.T) {
			p := testPolicy(false, false)
			if blocked(t, shellEvent("gh issue view 42"), p) || blocked(t, shellEvent(`echo "/approve plan"`), p) {
				t.Error("blocked")
			}
		}},
		guardCase{"policy survives a json round trip", func(t *testing.T) {
			original := testPolicy(false, false)
			text, err := dartjson.Encode(original.ToJSON())
			if err != nil {
				t.Fatal(err)
			}
			value, err := dartjson.Decode(text)
			if err != nil {
				t.Fatal(err)
			}
			back, err := LoadPolicy(value)
			if err != nil {
				t.Fatal(err)
			}
			again, _ := dartjson.Encode(back.ToJSON())
			if again != text {
				t.Errorf("got %s\nwant %s", again, text)
			}
		}},
	)
	return cases
}

func TestEveryDartGuardTestIsPorted(t *testing.T) {
	names := map[string]bool{}
	cases := guardCases()
	for _, c := range cases {
		if names[c.dart] {
			t.Errorf("duplicate case %q", c.dart)
		}
		names[c.dart] = true
	}
	if len(names) != dartGuardTests {
		t.Errorf("%d Dart guard tests ported, want %d", len(names), dartGuardTests)
	}
	t.Logf("guard_test.dart: %d tests ported", len(names))
}

func TestGuard(t *testing.T) {
	for _, c := range guardCases() {
		t.Run(c.dart, c.run)
	}
}
