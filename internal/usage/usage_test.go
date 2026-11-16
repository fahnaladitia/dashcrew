package usage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fixture is a project folder, a Claude Code home and a Codex home in one temp root.
type fixture struct {
	t                    *testing.T
	project, claude, cdx string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{t: t, project: filepath.Join(root, "work", "my_app"), claude: filepath.Join(root, "claude"),
		cdx: filepath.Join(root, "codex")}
	f.write(filepath.Join(f.project, ".ai", "project.yaml"), "toolkit: 1\n")
	return f
}

func (f fixture) write(path, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// lines joins JSON values, one per line.
func lines(t *testing.T, values ...any) string {
	t.Helper()
	var b strings.Builder
	for _, v := range values {
		if s, ok := v.(string); ok {
			b.WriteString(s + "\n")
			continue
		}
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

type obj = map[string]any

func assistant(id string, in, write, read, out int, content ...obj) obj {
	if content == nil {
		content = []obj{{"type": "text", "text": "ok"}}
	}
	return obj{"type": "assistant", "timestamp": "2026-11-10T09:15:00Z", "message": obj{
		"id": id, "model": "claude-opus-5-5", "role": "assistant", "content": content,
		"usage": obj{"input_tokens": in, "cache_creation_input_tokens": write, "cache_read_input_tokens": read,
			"output_tokens": out}}}
}

func user(content any) obj {
	return obj{"type": "user", "timestamp": "2026-11-10T09:14:00Z", "message": obj{"role": "user", "content": content}}
}

func toolUse(name string, input obj) obj {
	return obj{"type": "tool_use", "id": "toolu_1", "name": name, "input": input}
}

func (f fixture) claudeSession(dir, id string, body string) string {
	path := filepath.Join(f.claude, "projects", encode(dir), id+".jsonl")
	f.write(path, body)
	return path
}

func (f fixture) load(q Query) []Session {
	f.t.Helper()
	if q.Dirs == nil {
		q.Dirs = []string{f.project}
	}
	sessions, err := Load(Places{ClaudeHome: f.claude, CodexHome: f.cdx}, q)
	if err != nil {
		f.t.Fatal(err)
	}
	return sessions
}

func role(t *testing.T, s Session, name string) Role {
	t.Helper()
	for _, r := range s.Roles {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no role %q in %+v", name, s.Roles)
	return Role{}
}

func TestEncode(t *testing.T) {
	if got := encode("/Users/me/MyWorks/expense_tracker.v2"); got != "-Users-me-MyWorks-expense-tracker-v2" {
		t.Fatal(got)
	}
}

func TestClaudeMainSession(t *testing.T) {
	f := newFixture(t)
	cwd := obj{"type": "system", "cwd": f.project, "sessionId": "s1", "timestamp": "2026-11-10T09:14:00Z"}
	f.claudeSession(f.project, "fccf7522-aaaa", lines(t,
		cwd,
		user("<command-name>/clear</command-name>"),
		user("<command-name>/tdd</command-name>\n<command-args></command-args>"),
		// One response over three lines: usage counted once.
		assistant("msg_1", 2, 50000, 20000, 10),
		assistant("msg_1", 2, 50000, 20000, 10, toolUse("Skill", obj{"skill": "checkpoint", "args": "x"})),
		assistant("msg_1", 2, 50000, 20000, 10, toolUse("Read", obj{"file_path": f.project + "/.claude/skills/tdd/SKILL.md"})),
		// Writing, editing or searching a SKILL.md is not loading it.
		assistant("msg_1", 2, 50000, 20000, 10, toolUse("Bash", obj{"command": "grep -n x .claude/skills/tdd/SKILL.md"})),
		assistant("msg_1", 2, 50000, 20000, 10, toolUse("Edit", obj{"file_path": f.project + "/.claude/skills/plan/SKILL.md"})),
		`{"type": "assistant", broken`,
		assistant("msg_2", 3, 1000, 70000, 200),
		obj{"type": "assistant", "message": obj{"id": "msg_s", "model": "<synthetic>", "content": []obj{},
			"usage": obj{"input_tokens": 0, "output_tokens": 0}}},
	))
	f.write(filepath.Join(f.project, ".claude", "skills", "tdd", "SKILL.md"), "# tdd\n")

	sessions := f.load(Query{})
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions", len(sessions))
	}
	s := sessions[0]
	if s.Agent != "claude" || s.ID != "fccf7522-aaaa" || s.Model != "claude-opus-5-5" || s.Skipped != 1 {
		t.Fatalf("%+v", s)
	}
	if !s.Start.Equal(time.Date(2026, 11, 10, 9, 14, 0, 0, time.UTC)) {
		t.Fatalf("start %v", s.Start)
	}
	if s.StartContext != 70002 {
		t.Fatalf("start context %d", s.StartContext)
	}
	main := role(t, s, "main")
	want := Tokens{Responses: 2, Input: 5, CacheWrite: 51000, CacheRead: 90000, Output: 210}
	if main.Tokens != want || main.Runs != 1 {
		t.Fatalf("main %+v", main)
	}
	if !reflect.DeepEqual(main.Skills, map[string]int{"checkpoint": 1, "tdd": 2}) {
		t.Fatalf("skills %v", main.Skills)
	}
}

func TestClaudeSubagentRoles(t *testing.T) {
	f := newFixture(t)
	cwd := obj{"type": "system", "cwd": f.project, "timestamp": "2026-11-10T09:14:00Z"}
	f.claudeSession(f.project, "s1", lines(t, cwd, assistant("m1", 1, 10, 10, 1)))
	sub := filepath.Join(f.claude, "projects", encode(f.project), "s1", "subagents")
	agent := func(id, agentType, prompt string, out int) {
		f.write(filepath.Join(sub, "agent-"+id+".meta.json"), `{"agentType":"`+agentType+`","description":"x"}`)
		f.write(filepath.Join(sub, "agent-"+id+".jsonl"), lines(t,
			obj{"type": "user", "isSidechain": true, "agentId": id, "cwd": f.project, "message": obj{"role": "user", "content": prompt}},
			assistant("m-"+id, 1, 100, 1000, out, toolUse("Read", obj{"file_path": "/x/.agents/skills/review-checklist/SKILL.md"})),
		))
	}
	agent("a1", "reviewer", "Review the diff.", 5)
	agent("a2", "general-purpose", "You are the read-only `reviewer` in /p/.agents/agents/reviewer.md (read it).", 7)
	agent("a3", "general-purpose", "Find the router.", 9)
	agent("a4", "explorer", "Map the state layer.", 11)

	s := f.load(Query{})[0]
	names := []string{}
	for _, r := range s.Roles {
		names = append(names, r.Name)
	}
	if strings.Join(names, " ") != "main explorer other:general-purpose reviewer" {
		t.Fatalf("roles %v", names)
	}
	reviewer := role(t, s, "reviewer")
	if reviewer.Runs != 2 || reviewer.Output != 12 || reviewer.Responses != 2 || reviewer.Skills["review-checklist"] != 2 {
		t.Fatalf("reviewer %+v", reviewer)
	}
	if role(t, s, "other:general-purpose").Output != 9 {
		t.Fatal("other")
	}
}

func TestClaudeOldSidechainLines(t *testing.T) {
	f := newFixture(t)
	f.claudeSession(f.project, "s1", lines(t,
		obj{"type": "system", "cwd": f.project, "timestamp": "2026-11-10T09:14:00Z"},
		assistant("m1", 1, 10, 10, 1),
		obj{"type": "user", "isSidechain": true, "agentId": "x1", "message": obj{"role": "user",
			"content": []obj{{"type": "text", "text": "Act as .claude/agents/explorer.md and map lib/."}}}},
		func() obj { a := assistant("m2", 1, 10, 10, 4); a["isSidechain"] = true; a["agentId"] = "x1"; return a }(),
		obj{"type": "user", "isSidechain": true, "agentId": "x2", "message": obj{"role": "user", "content": "Summarize."}},
		func() obj { a := assistant("m3", 1, 10, 10, 6); a["isSidechain"] = true; a["agentId"] = "x2"; return a }(),
	))
	s := f.load(Query{})[0]
	if role(t, s, "main").Output != 1 || role(t, s, "explorer").Output != 4 || role(t, s, "other:subagent").Output != 6 {
		t.Fatalf("%+v", s.Roles)
	}
}

func TestClaudeFolderFilters(t *testing.T) {
	f := newFixture(t)
	// my_app and my-app share an encoded folder; only lines whose cwd is in the project count.
	other := filepath.Join(filepath.Dir(f.project), "my-app")
	f.claudeSession(f.project, "mine", lines(t, obj{"type": "system", "cwd": f.project, "timestamp": "2026-11-10T09:14:00Z"}, assistant("m", 1, 1, 1, 1)))
	f.claudeSession(other, "theirs", lines(t, obj{"type": "system", "cwd": other, "timestamp": "2026-11-10T09:14:00Z"}, assistant("m", 1, 1, 1, 1)))
	// A subfolder of the project has its own encoded folder.
	sub := filepath.Join(f.project, "packages", "core")
	f.claudeSession(sub, "sub", lines(t, obj{"type": "system", "cwd": sub, "timestamp": "2026-11-11T09:14:00Z"}, assistant("m", 1, 1, 1, 1)))
	// No response: left out.
	f.claudeSession(f.project, "empty", lines(t, obj{"type": "system", "cwd": f.project, "timestamp": "2026-11-12T09:14:00Z"}))

	ids := func(sessions []Session) string {
		out := []string{}
		for _, s := range sessions {
			out = append(out, s.ID)
		}
		return strings.Join(out, " ")
	}
	if got := ids(f.load(Query{})); got != "sub mine" {
		t.Fatalf("got %q", got)
	}
	if got := ids(f.load(Query{Since: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)})); got != "sub" {
		t.Fatalf("since: got %q", got)
	}
	if got := ids(f.load(Query{Session: "mi"})); got != "mine" {
		t.Fatalf("session: got %q", got)
	}
	if got := ids(f.load(Query{Agent: "codex"})); got != "" {
		t.Fatalf("agent: got %q", got)
	}
}

func (f fixture) codexRollout(name, body string) {
	f.write(filepath.Join(f.cdx, "sessions", "2026", "11", "11", name), body)
}

func tokenCount(totalIn, cached, out, lastIn int) obj {
	return obj{"type": "event_msg", "timestamp": "2026-11-11T14:03:00Z", "payload": obj{"type": "token_count", "info": obj{
		"total_token_usage": obj{"input_tokens": totalIn, "cached_input_tokens": cached, "output_tokens": out, "reasoning_output_tokens": 0},
		"last_token_usage":  obj{"input_tokens": lastIn, "cached_input_tokens": 0, "output_tokens": 1}}}}
}

func TestCodexRollout(t *testing.T) {
	f := newFixture(t)
	f.codexRollout("rollout-2026-11-11T14-02-00-0a1b.jsonl", lines(t,
		obj{"type": "session_meta", "timestamp": "2026-11-11T14:02:00Z", "payload": obj{"id": "0a1b2c3d-1111", "cwd": f.project, "source": "cli"}},
		obj{"type": "turn_context", "payload": obj{"cwd": f.project, "model": "gpt-5-codex"}},
		obj{"type": "event_msg", "payload": obj{"type": "token_count", "info": nil}},
		tokenCount(31000, 20000, 500, 31000),
		obj{"type": "response_item", "payload": obj{"type": "function_call", "name": "shell",
			"arguments": `{"command":["bash","-lc","cat .agents/skills/tdd/SKILL.md"]}`}},
		obj{"type": "response_item", "payload": obj{"type": "function_call", "name": "shell",
			"arguments": `{"command":["bash","-lc","sed -n '1,80p' .agents/skills/plan/SKILL.md"]}`}},
		obj{"type": "response_item", "payload": obj{"type": "function_call", "name": "shell",
			"arguments": `{"command":["bash","-lc","rg -n review .agents/skills/verify/SKILL.md; ls .agents/skills/x/SKILL.md"]}`}},
		`not json`,
		tokenCount(90000, 70000, 1500, 40000),
	))
	f.codexRollout("rollout-2026-11-11T15-00-00-9999.jsonl", lines(t,
		obj{"type": "session_meta", "timestamp": "2026-11-11T15:00:00Z", "payload": obj{"id": "elsewhere", "cwd": "/somewhere/else"}},
		tokenCount(10, 0, 1, 10),
	))
	f.codexRollout("rollout-2026-11-11T16-00-00-7777.jsonl", lines(t,
		obj{"type": "session_meta", "timestamp": "2026-11-11T16:00:00Z", "payload": obj{"id": "sub-1", "cwd": f.project,
			"source": obj{"subagent": "review"}}},
		obj{"type": "event_msg", "payload": obj{"type": "user_message", "message": "You are the reviewer in .codex/agents/reviewer.toml."}},
		tokenCount(100, 50, 10, 100),
	))

	sessions := f.load(Query{})
	if len(sessions) != 2 {
		t.Fatalf("got %+v", sessions)
	}
	sub, s := sessions[0], sessions[1]
	if s.Agent != "codex" || s.ID != "0a1b2c3d-1111" || s.Model != "gpt-5-codex" || s.StartContext != 31000 || s.Skipped != 1 {
		t.Fatalf("%+v", s)
	}
	main := role(t, s, "main")
	if main.Tokens != (Tokens{Responses: 2, Input: 20000, CacheRead: 70000, Output: 1500}) || !reflect.DeepEqual(main.Skills, map[string]int{"plan": 1, "tdd": 1}) {
		t.Fatalf("main %+v", main)
	}
	if r := role(t, sub, "reviewer"); r.Input != 50 || r.CacheRead != 50 || len(sub.Roles) != 1 {
		t.Fatalf("subagent %+v", sub.Roles)
	}
}

func TestText(t *testing.T) {
	sessions := []Session{{
		Agent: "claude", ID: "fccf7522-aaaa-bbbb", Model: "claude-opus-5-5", StartContext: 76100,
		Start: time.Date(2026, 11, 10, 9, 14, 0, 0, time.UTC),
		Roles: []Role{
			{Name: "main", Runs: 1, Tokens: Tokens{Responses: 416, Input: 1200, CacheWrite: 410300, CacheRead: 18_400_000, Output: 96000},
				Skills: map[string]int{"checkpoint": 1, "tdd": 2}},
			{Name: "reviewer", Runs: 3, Tokens: Tokens{Responses: 9, Input: 40, CacheWrite: 88200, CacheRead: 402000, Output: 6100}},
		},
	}, {
		Agent: "codex", ID: "0a1b2c3d-1111", Model: "gpt-5-codex", StartContext: 31000, Skipped: 2,
		Start: time.Date(2026, 11, 11, 14, 2, 0, 0, time.UTC),
		Roles: []Role{{Name: "main", Runs: 1, Tokens: Tokens{Responses: 2, Input: 20000, CacheRead: 70000, Output: 1500}}},
	}}
	var b bytes.Buffer
	Text(&b, sessions, time.UTC)
	want := `claude  fccf7522  2026-11-10 09:14  claude-opus-5-5  start 76.1k
  role      runs  responses  input  cache-write  cache-read  output
  main         1        416   1.2k       410.3k       18.4M   96.0k
  reviewer     3          9     40        88.2k      402.0k    6.1k
  skills    main: checkpoint, tdd x2
codex  0a1b2c3d  2026-11-11 14:02  gpt-5-codex  start 31.0k  skipped 2 lines
  role  runs  responses  input  cache-write  cache-read  output
  main     1          2  20.0k            0       70.0k    1.5k
total  2 sessions  input 21.2k  cache-write 498.5k  cache-read 18.9M  output 103.6k
`
	if b.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", b.String(), want)
	}
}

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	err := JSON(&b, []Session{{Agent: "codex", ID: "x", Start: time.Date(2026, 11, 11, 14, 2, 0, 0, time.UTC),
		Roles: []Role{{Name: "main", Runs: 1, Tokens: Tokens{Responses: 1, Input: 3}, Skills: map[string]int{"tdd": 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	r := got[0]["roles"].([]any)[0].(map[string]any)
	if got[0]["start"] != "2026-11-11T14:02:00Z" || r["input"] != float64(3) || r["cache_write"] != float64(0) ||
		r["skills"].(map[string]any)["tdd"] != float64(1) {
		t.Fatalf("%s", b.String())
	}
}
