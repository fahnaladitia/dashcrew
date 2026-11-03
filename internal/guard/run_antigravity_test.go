package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunAnswersAntigravityOnEveryAllowPath pins that Antigravity gets an explicit decision even where the guard
// allows without evaluating (no policy found, a policy that does not parse): agy fails closed on a missing one.
func TestRunAnswersAntigravityOnEveryAllowPath(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"policy missing":     {filepath.Join(dir, "missing.json")},
		"policy not json":    {broken},
		"policy is a folder": {dir},
	} {
		var stdout, stderr bytes.Buffer
		input := `{"toolCall":{"name":"run_command","args":{"CommandLine":"git status"}},"workspacePaths":["/nowhere"]}`

		code := Run(args, strings.NewReader(input), &stdout, &stderr, Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return dir, nil }})

		if code != 0 || strings.TrimSpace(stdout.String()) != `{"decision":"allow"}` || stderr.Len() != 0 {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunAllowsInputThatIsNotJSONWithoutAnswer(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run(nil, strings.NewReader("not json"), &stdout, &stderr, Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return t.TempDir(), nil }})

	if code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

// TestAntigravityWriteToolsBeyondTheDocumentedThree checks the write tools of agy's own permission vetting by their
// path and new text; their arguments are undocumented, so the usual keys are read.
func TestAntigravityWriteToolsBeyondTheDocumentedThree(t *testing.T) {
	imp := jsonString(vendorImport)
	for _, c := range []struct{ name, args string }{
		{"create_file", `{"file_path": "lib/feature/a.dart", "content": ` + imp + `}`},
		{"write_file", `{"path": "lib/feature/a.dart", "content": ` + imp + `}`},
		{"file_write", `{"AbsolutePath": "lib/feature/a.dart", "content": ` + imp + `}`},
		{"edit_file", `{"TargetFile": "lib/feature/a.dart", "CodeContent": ` + imp + `}`},
		{"str_replace_editor", `{"command": "str_replace", "path": "lib/feature/a.dart", "old_str": "x", "new_str": ` + imp + `}`},
		{"str_replace_editor", `{"command": "create", "path": "lib/feature/a.dart", "file_text": ` + imp + `}`},
	} {
		if !blockedAll(t, allFromHook(t, antigravityHook(c.name, c.args))) {
			t.Errorf("%s %s allowed", c.name, c.args)
		}
	}
	if blockedAll(t, allFromHook(t, antigravityHook("str_replace_editor", `{"command": "str_replace", "path": "lib/feature/a.dart", "old_str": `+imp+`, "new_str": ""}`))) {
		t.Error("removing a vendor import is blocked")
	}
	for _, name := range []string{"delete_file", "file_delete"} {
		events := allFromHook(t, antigravityHook(name, `{"path": "lib/feature/a.dart"}`))
		if len(events) != 1 || events[0].Kind != Write || events[0].FilePath != "lib/feature/a.dart" || events[0].Content != "" {
			t.Errorf("%s: %+v", name, events)
		}
	}
	if events := allFromHook(t, antigravityHook("create_file", `{"content": "x"}`)); len(events) != 1 || events[0].Kind != Other {
		t.Errorf("no path: %+v", events)
	}
}

func TestAntigravityToolArgumentsOfOtherAgentFormats(t *testing.T) {
	imp := jsonString(vendorImport)
	if !blockedAll(t, allFromHook(t, antigravityHook("edit_file", `{"target_file": "lib/feature/a.dart", "instructions": "add", "code_edit": `+imp+`}`))) {
		t.Error("edit_file with target_file and code_edit allowed")
	}
	if !blockedAll(t, allFromHook(t, antigravityHook("write_file", `{"filePath": "lib/feature/a.dart", "content": `+imp+`}`))) {
		t.Error("write_file with filePath allowed")
	}
	events := allFromHook(t, antigravityHook("str_replace_editor", `{"command": "view", "path": "lib/feature/a.dart"}`))
	if len(events) != 1 || events[0].Kind != Other {
		t.Errorf("str_replace_editor view is a write: %+v", events)
	}
}

// TestAnAntigravityPolicyArgumentHoldsWhenTheWorkspaceIsAParentFolder pins P3h: with the policy passed by the hook
// command, a workspace opened one folder above the project still gets the project's rules.
func TestAnAntigravityPolicyArgumentHoldsWhenTheWorkspaceIsAParentFolder(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "app")
	policy := filepath.Join(project, ".ai", "guard.json")
	if err := os.MkdirAll(filepath.Dir(policy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte(corpusPolicies["default"]), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	input := `{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}},"workspacePaths":[` + jsonString(parent) + `]}`

	Run([]string{policy}, strings.NewReader(input), &stdout, &stderr, Env{LookupEnv: func(string) (string, bool) { return "", false }, Getwd: func() (string, error) { return filepath.Join(project, ".agents"), nil }})

	if !strings.HasPrefix(stdout.String(), `{"decision":"deny"`) {
		t.Errorf("stdout %q", stdout.String())
	}
}
