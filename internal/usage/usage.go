// Package usage reads the Claude Code and Codex transcripts of a project and sums their tokens per session and per
// role (main, reviewer, explorer, other subagents), with the skills each role loaded. It reads only.
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Places are the agents' home folders: `$CLAUDE_CONFIG_DIR` or `~/.claude`, and `$CODEX_HOME` or `~/.codex`.
// An empty place is not read.
type Places struct {
	ClaudeHome, CodexHome string
}

// Query picks the sessions to report.
type Query struct {
	// Dirs are the project folders: the checkout and its worktrees. A session counts when its cwd is one of them or
	// inside one.
	Dirs []string
	// Since leaves out sessions that started before it; zero keeps all.
	Since time.Time
	// Agent is "claude", "codex" or empty for both.
	Agent string
	// Session keeps the sessions whose id starts with it; empty keeps all.
	Session string
}

// Tokens are the tokens of one or more model responses. Input excludes cached tokens.
type Tokens struct {
	Responses                            int
	Input, CacheWrite, CacheRead, Output int64
}

func (t *Tokens) add(o Tokens) {
	t.Responses += o.Responses
	t.Input += o.Input
	t.CacheWrite += o.CacheWrite
	t.CacheRead += o.CacheRead
	t.Output += o.Output
}

// Role is what one role used in a session: main, a dashcrew role, or `other:<agent type>`.
type Role struct {
	Name string
	// Runs is how many times the role ran: 1 for main, one per subagent spawn otherwise.
	Runs int
	Tokens
	// Skills counts each skill the role loaded.
	Skills map[string]int
}

// Session is one agent session.
type Session struct {
	Agent, ID, Model string
	Start            time.Time
	// StartContext is the context of the first main response: what the agent loaded before doing anything.
	StartContext int64
	// Roles holds main first, then the other roles by name.
	Roles []Role
	// Skipped counts the lines that did not parse.
	Skipped int
}

// project is what a parse needs to know about the project.
type project struct {
	dirs   []string
	roles  map[string]bool
	skills map[string]bool
}

// under is true when path is one of the project dirs or inside one.
func (p project) under(path string) bool {
	if path == "" {
		return false
	}
	path = filepath.Clean(path)
	for _, dir := range p.dirs {
		if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Load finds and parses the sessions q asks for, newest first.
func Load(places Places, q Query) ([]Session, error) {
	p := project{roles: map[string]bool{"explorer": true, "reviewer": true}, skills: map[string]bool{}}
	for _, dir := range q.Dirs {
		p.dirs = append(p.dirs, filepath.Clean(dir))
		for _, folder := range []string{".claude/agents", ".agents/agents", ".codex/agents"} {
			addNames(p.roles, filepath.Join(dir, filepath.FromSlash(folder)), true)
		}
		for _, folder := range []string{".claude/skills", ".agents/skills"} {
			addNames(p.skills, filepath.Join(dir, filepath.FromSlash(folder)), false)
		}
	}
	for _, home := range []string{places.ClaudeHome, places.CodexHome} {
		if home != "" {
			addNames(p.skills, filepath.Join(home, "skills"), false)
		}
	}

	var sessions []Session
	keep := func(s Session) {
		if s.responses() == 0 || (!q.Since.IsZero() && s.Start.Before(q.Since)) || !strings.HasPrefix(s.ID, q.Session) {
			return
		}
		sessions = append(sessions, s)
	}
	if places.ClaudeHome != "" && (q.Agent == "" || q.Agent == "claude") {
		files, err := claudeFiles(places.ClaudeHome, p.dirs, q.Since)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			s, ok, err := parseClaude(file, p)
			if err != nil {
				return nil, err
			}
			if ok {
				keep(s)
			}
		}
	}
	if places.CodexHome != "" && (q.Agent == "" || q.Agent == "codex") {
		files, err := codexFiles(places.CodexHome, q.Since)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			s, ok, err := parseCodex(file, p)
			if err != nil {
				return nil, err
			}
			if ok {
				keep(s)
			}
		}
	}
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].Start.After(sessions[j].Start) })
	return sessions, nil
}

func (s Session) responses() int {
	n := 0
	for _, r := range s.Roles {
		n += r.Responses
	}
	return n
}

// addNames adds the names in dir: subfolders, or with files the `.md` and `.toml` file names without extension.
func addNames(names map[string]bool, dir string, files bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		switch {
		case !files && e.IsDir():
			names[e.Name()] = true
		case files && !e.IsDir():
			if ext := filepath.Ext(e.Name()); ext == ".md" || ext == ".toml" {
				names[strings.TrimSuffix(e.Name(), ext)] = true
			}
		}
	}
}

// roles sums runs by role name.
type roles map[string]*Role

func (rs roles) add(name string, runs int, t Tokens, skills map[string]int) {
	r := rs[name]
	if r == nil {
		r = &Role{Name: name, Skills: map[string]int{}}
		rs[name] = r
	}
	r.Runs += runs
	r.add(t)
	for skill, n := range skills {
		r.Skills[skill] += n
	}
}

// list is main first, then the other roles by name; an empty skill map becomes nil.
func (rs roles) list() []Role {
	out := make([]Role, 0, len(rs))
	for _, r := range rs {
		if len(r.Skills) == 0 {
			r.Skills = nil
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Name == "main") != (out[j].Name == "main") {
			return out[i].Name == "main"
		}
		return out[i].Name < out[j].Name
	})
	return out
}

var (
	roleFile  = regexp.MustCompile(`(?:\.claude/agents/|\.agents/agents/)([A-Za-z0-9_-]+)\.md|\.codex/agents/([A-Za-z0-9_-]+)\.toml`)
	skillFile = regexp.MustCompile(`skills/([A-Za-z0-9_.:-]+)/SKILL\.md`)
	// skillShellRead is a shell command that prints a SKILL.md: a read verb, then the path in the same command.
	skillShellRead = regexp.MustCompile(`(?:^|[\s"';&|(])(?:cat|head|tail|nl|less|bat|sed)\s[^;&|"\n]*?skills/([A-Za-z0-9_.:-]+)/SKILL\.md`)
	command        = regexp.MustCompile(`<command-name>/?([^<\s]+)</command-name>`)
)

// roleOf names a subagent: its agent type when that is a role, else the role file its prompt names, else
// `other:<agent type>`.
func (p project) roleOf(agentType, prompt string) string {
	if p.roles[agentType] {
		return agentType
	}
	if m := roleFile.FindStringSubmatch(prompt); m != nil {
		return m[1] + m[2]
	}
	if agentType == "" {
		agentType = "subagent"
	}
	return "other:" + agentType
}

// skillReads counts the SKILL.md files that text, a read tool's input or a shell command, prints.
func skillReads(skills map[string]int, text []byte, pattern *regexp.Regexp) {
	for _, m := range pattern.FindAllSubmatch(text, -1) {
		skills[string(m[1])]++
	}
}

// eachLine calls f with each non-empty line of path.
func eachLine(path string, f func(line []byte)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	r := bufio.NewReaderSize(file, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if line = bytes.TrimSpace(line); len(line) > 0 {
			f(line)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// modifiedBefore is true when path was last written before since: a session that ended then cannot have started
// after it.
func modifiedBefore(info fs.FileInfo, since time.Time) bool {
	return !since.IsZero() && info.ModTime().Before(since)
}

func parseTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// jsonString is the string in raw, or empty.
func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}
