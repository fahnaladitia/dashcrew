package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// encode is the folder name Claude Code gives a working directory under `projects/`: every character outside
// [A-Za-z0-9] becomes `-`. Different paths can share one name, so a session's own cwd decides.
func encode(dir string) string {
	b := []byte(dir)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			b[i] = '-'
		}
	}
	return string(b)
}

// claudeFiles lists the main transcripts in the `projects/` folders of the dirs and of their subfolders.
func claudeFiles(home string, dirs []string, since time.Time) ([]string, error) {
	projects := filepath.Join(home, "projects")
	entries, err := os.ReadDir(projects)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() || !matchesAny(e.Name(), dirs) {
			continue
		}
		folder := filepath.Join(projects, e.Name())
		sessions, err := os.ReadDir(folder)
		if err != nil {
			return nil, err
		}
		for _, s := range sessions {
			if s.IsDir() || filepath.Ext(s.Name()) != ".jsonl" {
				continue
			}
			info, err := s.Info()
			if err != nil {
				return nil, err
			}
			if !modifiedBefore(info, since) {
				files = append(files, filepath.Join(folder, s.Name()))
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func matchesAny(name string, dirs []string) bool {
	for _, dir := range dirs {
		if encoded := encode(dir); name == encoded || strings.HasPrefix(name, encoded+"-") {
			return true
		}
	}
	return false
}

// claudeLine is the part of a transcript line the report reads.
type claudeLine struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	AgentID     string `json:"agentId"`
	CWD         string `json:"cwd"`
	Timestamp   string `json:"timestamp"`
	Message     *struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			Input      int64 `json:"input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
			Output     int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// claudeRun sums one main transcript, one subagent file, or the old-style sidechain lines of one agent.
type claudeRun struct {
	prompt string
	seen   map[string]bool
	first  int64
	tokens Tokens
	skills map[string]int
	model  string
}

func newClaudeRun() *claudeRun {
	return &claudeRun{seen: map[string]bool{}, first: -1, skills: map[string]int{}}
}

func (r *claudeRun) read(l claudeLine, p project) {
	m := l.Message
	if m == nil {
		return
	}
	switch l.Type {
	case "user":
		text := userText(m.Content)
		if r.prompt == "" {
			r.prompt = text
		}
		for _, match := range command.FindAllStringSubmatch(text, -1) {
			if p.skills[match[1]] {
				r.skills[match[1]]++
			}
		}
	case "assistant":
		if m.Model == "<synthetic>" {
			return
		}
		var blocks []contentBlock
		_ = json.Unmarshal(m.Content, &blocks)
		for _, b := range blocks {
			if b.Type != "tool_use" {
				continue
			}
			if b.Name == "Skill" {
				var in struct {
					Skill string `json:"skill"`
				}
				if json.Unmarshal(b.Input, &in) == nil && in.Skill != "" {
					r.skills[in.Skill]++
				}
				continue
			}
			if b.Name == "Read" {
				skillReads(r.skills, b.Input, skillFile)
			}
		}
		if m.Usage == nil || m.ID == "" || r.seen[m.ID] {
			return
		}
		r.seen[m.ID] = true
		if r.model == "" {
			r.model = m.Model
		}
		u := m.Usage
		if r.first < 0 {
			r.first = u.Input + u.CacheWrite + u.CacheRead
		}
		r.tokens.add(Tokens{Responses: 1, Input: u.Input, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead, Output: u.Output})
	}
}

// userText is the text of a user message: a string, or its text blocks joined.
func userText(content json.RawMessage) string {
	if s := jsonString(content); s != "" {
		return s
	}
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// parseClaude reads a main transcript and the subagent files next to it; false when its cwd is outside the project.
func parseClaude(path string, p project) (Session, bool, error) {
	s := Session{Agent: "claude", ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	main := newClaudeRun()
	sidechains := map[string]*claudeRun{}
	var order []string
	cwd := ""
	err := eachLine(path, func(line []byte) {
		var l claudeLine
		if json.Unmarshal(line, &l) != nil {
			s.Skipped++
			return
		}
		if cwd == "" && l.CWD != "" {
			cwd = l.CWD
		}
		if s.Start.IsZero() && l.Timestamp != "" {
			s.Start = parseTime(l.Timestamp)
		}
		run := main
		if l.IsSidechain {
			run = sidechains[l.AgentID]
			if run == nil {
				run = newClaudeRun()
				sidechains[l.AgentID] = run
				order = append(order, l.AgentID)
			}
		}
		run.read(l, p)
	})
	if err != nil {
		return Session{}, false, err
	}
	if !p.under(cwd) {
		return Session{}, false, nil
	}
	if s.Start.IsZero() {
		if info, err := os.Stat(path); err == nil {
			s.Start = info.ModTime()
		}
	}
	rs := roles{}
	rs.add("main", 1, main.tokens, main.skills)
	s.Model = main.model
	if main.first > 0 {
		s.StartContext = main.first
	}
	for _, id := range order {
		run := sidechains[id]
		rs.add(p.roleOf("", run.prompt), 1, run.tokens, run.skills)
	}

	subagents, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents", "agent-*.jsonl"))
	sort.Strings(subagents)
	for _, file := range subagents {
		run := newClaudeRun()
		var meta struct {
			AgentType string `json:"agentType"`
		}
		if data, err := os.ReadFile(strings.TrimSuffix(file, ".jsonl") + ".meta.json"); err == nil {
			_ = json.Unmarshal(data, &meta)
		}
		err := eachLine(file, func(line []byte) {
			var l claudeLine
			if json.Unmarshal(line, &l) != nil {
				s.Skipped++
				return
			}
			run.read(l, p)
		})
		if err != nil {
			return Session{}, false, err
		}
		rs.add(p.roleOf(meta.AgentType, run.prompt), 1, run.tokens, run.skills)
	}
	s.Roles = rs.list()
	return s, true, nil
}
