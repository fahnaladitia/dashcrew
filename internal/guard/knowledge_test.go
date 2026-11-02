package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// knowledgeFixture is a project whose `.ai/knowledge` links to a knowledge clone under home, a second link to that
// link (kb), and a link into the clone's features folder (docs/alias).
type knowledgeFixture struct {
	project, home, clone string
}

const knowledgePolicy = `{"schema":1,"blocked_git":["commit","push"],"block_formatters":true,"plans_dir":".ai/plans",` +
	`"role_capability":{"engineer":"write-code","explorer":"read","planner":"write-plans"},` +
	`"forbidden_imports":["package:vendor_sdk/"],"local_packages":[],"knowledge_clone":"~/.dashcrew/knowledge/notes"}`

func newKnowledgeFixture(t *testing.T) knowledgeFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := knowledgeFixture{project: filepath.Join(root, "project"), home: filepath.Join(root, "home")}
	f.clone = filepath.Join(f.home, ".dashcrew", "knowledge", "notes")
	for _, dir := range []string{
		filepath.Join(f.clone, "features"), filepath.Join(f.clone, "inbox"), filepath.Join(f.clone, "repos"),
		filepath.Join(f.project, ".ai"), filepath.Join(f.project, "lib"), filepath.Join(f.project, "docs"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for file, content := range map[string]string{
		filepath.Join(f.clone, "features", "share.md"): "share\n",
		filepath.Join(f.project, ".ai", "guard.json"):  knowledgePolicy,
	} {
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		filepath.Join(f.project, ".ai", "knowledge"): f.clone,
		filepath.Join(f.project, "kb"):               filepath.Join(".ai", "knowledge"),
		filepath.Join(f.project, "docs", "alias"):    filepath.Join(f.clone, "features"),
	} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// run sends one hook payload with the project as cwd and the fixture's home; true when the guard blocks it.
func (f knowledgeFixture) run(t *testing.T, payload string) (bool, string) {
	t.Helper()
	env := Env{
		LookupEnv: func(key string) (string, bool) {
			if key == "HOME" {
				return f.home, true
			}
			return "", false
		},
		Getwd: func() (string, error) { return f.project, nil },
	}
	payload = strings.NewReplacer("{clone}", f.clone, "{project}", f.project).Replace(payload)
	var stdout, stderr bytes.Buffer
	code := Run(nil, strings.NewReader(payload), &stdout, &stderr, env)
	return code == 2 || strings.Contains(stdout.String(), `"deny"`), stderr.String() + stdout.String()
}

func payloadJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func writePayload(tool, path, role string) string {
	payload := map[string]any{"tool_name": tool, "tool_input": map[string]any{"file_path": path, "content": "x", "new_string": "x"}}
	if role != "" {
		payload["agent_type"] = role
	}
	return payloadJSON(payload)
}

func bashPayload(command string) string {
	return payloadJSON(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}})
}

func TestKnowledgeWritesAreAllowedOnlyUnderInbox(t *testing.T) {
	f := newKnowledgeFixture(t)
	cases := []struct {
		path    string
		blocked bool
	}{
		{".ai/knowledge/features/share.md", true},
		{".ai/knowledge/features/new.md", true},
		{".ai/knowledge/README.md", true},
		{".ai/knowledge/new-folder/deep/x.md", true},
		{".ai/knowledge/inbox", false},
		{".ai/knowledge/inbox/2026-11-01-share.md", false},
		{".ai/knowledge/inbox/sub/x.md", false},
		{"{clone}/features/share.md", true},
		{"{clone}/inbox/x.md", false},
		{"{project}/.ai/knowledge/repos/notes-api.md", true},
		{".ai/knowledge/inbox/../features/share.md", true},
		{"lib/../.ai/knowledge/features/x.md", true},
		{".ai/knowledge/inbox/../../knowledge/features/x.md", true},
		{"kb/features/x.md", true},
		{"kb/inbox/x.md", false},
		{"docs/alias/x.md", true},
		{"docs/alias/../inbox/x.md", false},
		{"docs/alias/../repos/x.md", true},
		{"lib/../docs/alias/../README.md", true},
		{"lib/a.dart", false},
		{"docs/notes.md", false},
	}
	for _, c := range cases {
		for _, tool := range []string{"Write", "Edit"} {
			for _, role := range []string{"", "engineer"} {
				got, reason := f.run(t, writePayload(tool, c.path, role))
				if got != c.blocked {
					t.Errorf("%s %s as %q: blocked %v, want %v (%s)", tool, c.path, role, got, c.blocked, reason)
				}
				if got && !strings.Contains(reason, knowledgeReason) {
					t.Errorf("%s: reason %q", c.path, reason)
				}
			}
		}
	}
	if got, reason := f.run(t, writePayload("Write", ".ai/knowledge/inbox/x.md", "planner")); !got || !strings.Contains(reason, "may only write under .ai/plans/") {
		t.Errorf("a role's own limits still apply in inbox: %v %s", got, reason)
	}
	antigravity := `{"toolCall":{"name":"write_to_file","args":{"TargetFile":"{clone}/features/x.md","CodeContent":"x"}},"workspacePaths":["{project}"]}`
	if got, reason := f.run(t, antigravity); !got || !strings.Contains(reason, "deny") {
		t.Errorf("antigravity: %v %s", got, reason)
	}
}

func TestKnowledgePatchesAreCheckedPerFile(t *testing.T) {
	f := newKnowledgeFixture(t)
	for patch, want := range map[string]bool{
		"*** Begin Patch\n*** Add File: .ai/knowledge/features/x.md\n+x\n*** End Patch":                             true,
		"*** Begin Patch\n*** Add File: .ai/knowledge/inbox/x.md\n+x\n*** End Patch":                                false,
		"*** Begin Patch\n*** Delete File: .ai/knowledge/features/share.md\n*** End Patch":                          true,
		"*** Begin Patch\n*** Update File: .ai/knowledge/inbox/x.md\n*** Move to: kb/repos/x.md\n+x\n*** End Patch": true,
		"*** Begin Patch\n*** Add File: lib/a.md\n+x\n*** Add File: docs/alias/y.md\n+y\n*** End Patch":             true,
	} {
		got, reason := f.run(t, payloadJSON(map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": patch}}))
		if got != want {
			t.Errorf("%q: blocked %v, want %v (%s)", patch, got, want, reason)
		}
	}
}

func TestKnowledgeShellWrites(t *testing.T) {
	f := newKnowledgeFixture(t)
	for command, want := range map[string]bool{
		"echo x > .ai/knowledge/features/x.md":                         true,
		"echo x >| .ai/knowledge/features/x.md":                        true,
		"echo x &> kb/features/x.md":                                   true,
		"printf x 1>>docs/alias/x.md":                                  true,
		"echo x >> .ai/knowledge/inbox/p.md":                           false,
		"echo x > /dev/null 2>&1":                                      false,
		"cat .ai/knowledge/features/share.md":                          false,
		"grep -rn share .ai/knowledge < .ai/knowledge/README.md":       false,
		"echo x | tee .ai/knowledge/features/x.md":                     true,
		"echo x | tee -a /tmp/log ~/.dashcrew/knowledge/notes/repos/x": true,
		"tee .ai/knowledge/inbox/p.md < draft.md":                      false,
		"cp draft.md .ai/knowledge/features/":                          true,
		"cp -r drafts/. {clone}/repos":                                 true,
		"cp -t .ai/knowledge/features a.md":                            true,
		"cp --target-directory=kb/features a.md":                       true,
		"cp .ai/knowledge/features/share.md /tmp/share.md":             false,
		"cp draft.md .ai/knowledge/inbox/":                             false,
		"mv .ai/knowledge/features/share.md .ai/knowledge/inbox/":      true,
		"mv draft.md .ai/knowledge/inbox/":                             false,
		"rm -rf .ai/knowledge/features":                                true,
		"rm -f -- .ai/knowledge/inbox/old.md":                          false,
		"rm -rf .ai/knowledge/":                                        true,
		"touch kb/features/x.md":                                       true,
		"mkdir -p .ai/knowledge/features/sub":                          true,
		"ln -s ../x .ai/knowledge/features/x":                          true,
		"sed -i 's/a/b/' .ai/knowledge/features/share.md":              true,
		"sed -i.bak -e 's/a/b/' .ai/knowledge/features/share.md":       true,
		"sed 's/a/b/' .ai/knowledge/features/share.md":                 false,
		"dd if=/dev/zero of=.ai/knowledge/features/x bs=1 count=1":     true,
		"truncate -s 0 $HOME/.dashcrew/knowledge/notes/features/x.md":  true,
		"echo x > ${HOME}/.dashcrew/knowledge/notes/inbox/x.md":        false,
		"find .ai/knowledge/features -name '*.md' -delete":             true,
		"find .ai/knowledge/features -name '*.md'":                     false,
		"find . -name x -exec rm .ai/knowledge/features/share.md ';'":  true,
		"bash -c 'echo x > .ai/knowledge/features/x.md'":               true,
		"sudo -u root tee .ai/knowledge/features/x.md":                 true,
		"echo $(echo x > .ai/knowledge/features/x.md)":                 true,
		"cd .ai/knowledge && echo x > features/x.md":                   true,
		"cd .ai && cd knowledge && rm features/share.md":               true,
		"cd .ai/knowledge/inbox && echo x > p.md":                      false,
		"cd docs && echo x > notes.md":                                 false,
		"git -C .ai/knowledge status":                                  false,
	} {
		if got, reason := f.run(t, bashPayload(command)); got != want {
			t.Errorf("%q: blocked %v, want %v (%s)", command, got, want, reason)
		}
	}
	codex := payloadJSON(map[string]any{"tool_name": "exec_command", "tool_input": map[string]any{"command": []any{"rm", "-f", ".ai/knowledge/features/share.md"}}})
	if got, _ := f.run(t, codex); !got {
		t.Error("codex argv rm was allowed")
	}
}

// TestKnowledgeShellBlindSpots pins what the guard cannot see: the script's effect is decided by a program it does
// not parse. They are allowed, as before knowledge existed.
func TestKnowledgeShellBlindSpots(t *testing.T) {
	f := newKnowledgeFixture(t)
	for _, command := range []string{
		`python3 -c "open('.ai/knowledge/features/x.md', 'w')"`,
		"echo .ai/knowledge/features/share.md | xargs rm",
		"D=.ai/knowledge/features; echo x > $D/x.md",
		"perl -pi -e 's/a/b/' .ai/knowledge/features/share.md",
		"tar -xf notes.tar -C .ai/knowledge",
		"git -C .ai/knowledge checkout -- features",
	} {
		if got, _ := f.run(t, bashPayload(command)); got {
			t.Errorf("%q is now blocked; move it to TestKnowledgeShellWrites", command)
		}
	}
}

func TestACopiedKnowledgeFolderIsProtectedLikeTheClone(t *testing.T) {
	f := newKnowledgeFixture(t)
	link := filepath.Join(f.project, ".ai", "knowledge")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(link, "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{".ai/knowledge/features/x.md": true, ".ai/knowledge/inbox/x.md": false, "{clone}/repos/x.md": true} {
		if got, _ := f.run(t, writePayload("Write", path, "")); got != want {
			t.Errorf("%s: blocked %v, want %v", path, got, want)
		}
	}
}

func TestWithoutKnowledgeTheClonePathIsNotProtected(t *testing.T) {
	f := newKnowledgeFixture(t)
	policy := strings.Replace(knowledgePolicy, `,"knowledge_clone":"~/.dashcrew/knowledge/notes"`, "", 1)
	if err := os.WriteFile(filepath.Join(f.project, ".ai", "guard.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{writePayload("Write", ".ai/knowledge/features/x.md", ""), bashPayload("echo x > .ai/knowledge/features/x.md")} {
		if got, reason := f.run(t, payload); got {
			t.Errorf("blocked without knowledge: %s", reason)
		}
	}
}

func TestWriteRulesFollowLinksIntoTheProject(t *testing.T) {
	f := newKnowledgeFixture(t)
	import_ := `{"tool_name":"Write","tool_input":{"file_path":"src/a.dart","content":"import 'package:vendor_sdk/a.dart';"}}`
	if err := os.Symlink("lib", filepath.Join(f.project, "src")); err != nil {
		t.Fatal(err)
	}
	if got, reason := f.run(t, import_); !got || !strings.Contains(reason, "Do not import package:vendor_sdk/") {
		t.Errorf("a link to lib/ hides app code: %v %s", got, reason)
	}
	if got, _ := f.run(t, writePayload("Write", ".ai/plans/x.md", "planner")); got {
		t.Error("planner blocked in plans")
	}
}

func TestKnowledgeClonePolicyFieldIsOptionalButTyped(t *testing.T) {
	for text, want := range map[string]string{
		knowledgePolicy: "~/.dashcrew/knowledge/notes",
		strings.Replace(knowledgePolicy, `,"knowledge_clone":"~/.dashcrew/knowledge/notes"`, "", 1): "",
	} {
		p, err := PolicyFromJSON(text)
		if err != nil || p.KnowledgeClone != want {
			t.Errorf("%q, %v", p.KnowledgeClone, err)
		}
		if got := p.ToJSON().Has("knowledge_clone"); got != (want != "") {
			t.Errorf("ToJSON has knowledge_clone %v", got)
		}
	}
	if _, err := PolicyFromJSON(strings.Replace(knowledgePolicy, `"~/.dashcrew/knowledge/notes"`, "3", 1)); err == nil {
		t.Error("a knowledge_clone number was accepted")
	}
}

func TestRealPath(t *testing.T) {
	f := newKnowledgeFixture(t)
	for path, want := range map[string]string{
		"{project}/.ai/knowledge/features/share.md":   "{clone}/features/share.md",
		"{project}/kb/missing/../inbox/x":             "{clone}/inbox/x",
		"{project}/docs/alias/../repos":               "{clone}/repos",
		"{project}/nothing/here/../x":                 "{project}/nothing/x",
		"{project}/.ai/knowledge":                     "{clone}",
		"{project}/.ai/knowledge/features/share.md/x": "{clone}/features/share.md/x",
	} {
		expand := strings.NewReplacer("{project}", f.project, "{clone}", f.clone, "/", string(filepath.Separator)).Replace
		if got := realPath(expand(path)); got != expand(want) {
			t.Errorf("%s: %s", path, got)
		}
	}
	loop := filepath.Join(f.project, "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	if got := realPath(filepath.Join(loop, "x")); got == "" {
		t.Error("a link loop gave no path")
	}
}

// withSpecs adds specs to the fixture's clone: one file per status, a file without front matter, and a link from
// the draft folder to the approved file.
func (f knowledgeFixture) withSpecs(t *testing.T) knowledgeFixture {
	t.Helper()
	for path, status := range map[string]string{
		"specs/1.4.0/notes-sync/prd.md": "draft", "specs/1.4.0/notes-sync/trd.md": "review",
		"specs/1.3.0/todo/prd.md": "approved", "specs/1.3.0/todo/trd.md": "superseded",
	} {
		file := filepath.Join(f.clone, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("---\ntitle: x\nstatus: "+status+"\nrev: 1\n---\n\n- AC-1 x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.clone, "specs", "README.md"), []byte("# Specs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "1.3.0", "todo", "prd.md"), filepath.Join(f.clone, "specs", "1.4.0", "notes-sync", "old.md")); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSpecWritesFollowTheFileStatus(t *testing.T) {
	f := newKnowledgeFixture(t).withSpecs(t)
	cases := []struct {
		path   string
		reason string
	}{
		{".ai/knowledge/specs/1.4.0/notes-sync/prd.md", ""},
		{".ai/knowledge/specs/1.4.0/notes-sync/trd.md", ""},
		{".ai/knowledge/specs/1.5.0/shop-cart/prd.md", ""},
		{".ai/knowledge/specs/1.4.0/notes-sync/new.md", ""},
		{".ai/knowledge/specs/1.3.0/todo/prd.md", specReason},
		{".ai/knowledge/specs/1.3.0/todo/trd.md", specReason},
		{".ai/knowledge/specs/README.md", specReason},
		{".ai/knowledge/specs/1.3.0/todo", specReason},
		{".ai/knowledge/specs", specReason},
		{".ai/knowledge/specs/1.4.0/notes-sync/old.md", specReason},
		{"{clone}/specs/1.4.0/notes-sync/prd.md", ""},
		{"{clone}/specs/1.3.0/todo/prd.md", specReason},
		{"{project}/.ai/knowledge/specs/1.3.0/todo/prd.md", specReason},
		{"kb/specs/1.3.0/todo/prd.md", specReason},
		{"kb/specs/1.4.0/notes-sync/prd.md", ""},
		{".ai/knowledge/specs/1.4.0/notes-sync/../../1.3.0/todo/prd.md", specReason},
		{".ai/knowledge/inbox/../specs/1.4.0/notes-sync/prd.md", ""},
		{".ai/knowledge/specs/../features/share.md", knowledgeReason},
		{"docs/alias/../specs/1.3.0/todo/trd.md", specReason},
		{"docs/alias/../specs/1.4.0/notes-sync/trd.md", ""},
		{".ai/knowledge/inbox/x.md", ""},
		{".ai/knowledge/features/share.md", knowledgeReason},
	}
	for _, c := range cases {
		for _, tool := range []string{"Write", "Edit"} {
			got, reason := f.run(t, writePayload(tool, c.path, ""))
			if got != (c.reason != "") || (got && !strings.Contains(reason, c.reason)) {
				t.Errorf("%s %s: blocked %v (%s), want %q", tool, c.path, got, reason, c.reason)
			}
		}
	}
}

func TestSpecPatchesAndShellWrites(t *testing.T) {
	f := newKnowledgeFixture(t).withSpecs(t)
	for patch, want := range map[string]bool{
		"*** Begin Patch\n*** Add File: .ai/knowledge/specs/1.5.0/shop-cart/prd.md\n+x\n*** End Patch":     false,
		"*** Begin Patch\n*** Update File: .ai/knowledge/specs/1.4.0/notes-sync/prd.md\n+x\n*** End Patch": false,
		"*** Begin Patch\n*** Update File: .ai/knowledge/specs/1.3.0/todo/prd.md\n+x\n*** End Patch":       true,
		"*** Begin Patch\n*** Delete File: kb/specs/1.3.0/todo/trd.md\n*** End Patch":                      true,
	} {
		got, reason := f.run(t, payloadJSON(map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": patch}}))
		if got != want {
			t.Errorf("%q: blocked %v, want %v (%s)", patch, got, want, reason)
		}
	}
	for command, want := range map[string]bool{
		"echo x > .ai/knowledge/specs/1.5.0/shop-cart/prd.md":               false,
		"echo x >> .ai/knowledge/specs/1.4.0/notes-sync/trd.md":             false,
		"echo x >> .ai/knowledge/specs/1.3.0/todo/prd.md":                   true,
		"sed -i 's/approved/review/' .ai/knowledge/specs/1.3.0/todo/prd.md": true,
		"sed -i 's/a/b/' .ai/knowledge/specs/1.4.0/notes-sync/prd.md":       false,
		"rm .ai/knowledge/specs/1.3.0/todo/trd.md":                          true,
		"rm -rf .ai/knowledge/specs/1.3.0":                                  true,
		"mv .ai/knowledge/specs/1.3.0/todo/prd.md /tmp/x":                   true,
		"cp draft.md .ai/knowledge/specs/1.4.0/notes-sync/prd.md":           false,
		"cd .ai/knowledge/specs/1.3.0/todo && echo x > prd.md":              true,
		"cd .ai/knowledge/specs/1.4.0 && echo x > notes-sync/prd.md":        false,
		"tee ~/.dashcrew/knowledge/notes/specs/1.3.0/todo/trd.md < x":       true,
		"bash -c 'echo x > kb/specs/1.3.0/todo/prd.md'":                     true,
		"dashcrew spec revise .ai/knowledge/specs/1.3.0/todo":               true,
		"rtk dashcrew spec revise specs/1.3.0/todo":                         true,
		"dashcrew spec lint .ai/knowledge/specs":                            false,
		"dashcrew spec new shop-cart --release 1.5.0":                       false,
		"mkdir -p .ai/knowledge/specs/1.5.0/shop-cart":                      false,
		"cat .ai/knowledge/specs/1.3.0/todo/prd.md":                         false,
		"echo x > .ai/knowledge/specs/1.4.0/notes-sync/old.md":              true,
	} {
		got, reason := f.run(t, bashPayload(command))
		if got != want {
			t.Errorf("%q: blocked %v, want %v (%s)", command, got, want, reason)
		}
		if got && strings.Contains(command, "specs/1.3.0") && !strings.Contains(command, "revise") && !strings.Contains(reason, specReason) {
			t.Errorf("%q: reason %s", command, reason)
		}
	}
	if _, reason := f.run(t, bashPayload("dashcrew spec revise .ai/knowledge/specs/1.3.0/todo")); !strings.Contains(reason, specReviseReason) {
		t.Errorf("revise reason: %s", reason)
	}
}

func TestSpecReasonNamesRevise(t *testing.T) {
	if !strings.Contains(specReason, "dashcrew spec revise") {
		t.Error(specReason)
	}
}

// TestPatchMovesCheckTheSource pins that a moved file is removed from its old path: the source passes the same write
// rules as the destination, so a protected page or spec cannot be moved into the inbox.
func TestPatchMovesCheckTheSource(t *testing.T) {
	f := newKnowledgeFixture(t).withSpecs(t)
	for _, c := range []struct {
		patch, reason string
	}{
		{"*** Begin Patch\n*** Update File: .ai/knowledge/specs/1.3.0/todo/prd.md\n*** Move to: .ai/knowledge/inbox/prd.md\n+x\n*** End Patch", specReason},
		{"*** Begin Patch\r\n*** Update File: .ai/knowledge/specs/1.3.0/todo/prd.md\r\n*** Move to: .ai/knowledge/inbox/prd.md\r\n+x\r\n*** End Patch", specReason},
		{"*** Begin Patch\n*** Update File: .ai/knowledge/features/share.md\n*** Move to: .ai/knowledge/inbox/share.md\n+x\n*** End Patch", knowledgeReason},
		{"*** Begin Patch\n*** Update File: kb/features/share.md\n*** Move to: lib/share.md\n*** End Patch", knowledgeReason},
		{"*** Begin Patch\n*** Update File: .ai/knowledge/inbox/a.md\n*** Move to: .ai/knowledge/inbox/b.md\n+x\n*** End Patch", ""},
		{"*** Begin Patch\n*** Update File: .ai/knowledge/inbox/a.md\n*** Move to: .ai/knowledge/inbox/sub/a.md\n*** End Patch", ""},
		{"*** Begin Patch\n*** Update File: .ai/knowledge/specs/1.4.0/notes-sync/prd.md\n*** Move to: .ai/knowledge/specs/1.4.0/notes-sync/old-prd.md\n*** End Patch", ""},
		{"*** Begin Patch\n*** Update File: lib/a.md\n*** Move to: lib/b.md\n+x\n*** End Patch", ""},
		{"*** Begin Patch\n*** Delete File: .ai/knowledge/specs/1.3.0/todo/prd.md\n*** End Patch", specReason},
		{"*** Begin Patch\n*** Delete File: .ai/knowledge/specs/1.4.0/notes-sync/prd.md\n*** End Patch", ""},
		{"*** Begin Patch\n*** Delete File: .ai/knowledge/inbox/x.md\n*** End Patch", ""},
	} {
		got, reason := f.run(t, payloadJSON(map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": c.patch}}))
		if got != (c.reason != "") || (got && !strings.Contains(reason, c.reason)) {
			t.Errorf("%q: blocked %v (%s), want %q", c.patch, got, reason, c.reason)
		}
	}
	if got, reason := f.run(t, payloadJSON(map[string]any{"tool_name": "apply_patch", "agent_type": "planner",
		"tool_input": map[string]any{"command": "*** Begin Patch\n*** Add File: lib/a.md\n*** Move to: .ai/plans/a.md\n+x\n*** End Patch"}})); !got || !strings.Contains(reason, "may only write under .ai/plans/") {
		t.Errorf("a planner moving a file out of lib/: %v %s", got, reason)
	}
}
