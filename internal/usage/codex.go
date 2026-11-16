package usage

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// codexFiles lists the rollouts under `sessions/` and `archived_sessions/`.
func codexFiles(home string, since time.Time) ([]string, error) {
	var files []string
	for _, folder := range []string{"sessions", "archived_sessions"} {
		root := filepath.Join(home, folder)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) && path == root {
					return filepath.SkipDir
				}
				return err
			}
			if d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || filepath.Ext(d.Name()) != ".jsonl" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !modifiedBefore(info, since) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexUsage struct {
	Input  int64 `json:"input_tokens"`
	Cached int64 `json:"cached_input_tokens"`
	Output int64 `json:"output_tokens"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`
	Model     string          `json:"model"`
	Source    json.RawMessage `json:"source"`
	Message   json.RawMessage `json:"message"`
	Info      *struct {
		Total *codexUsage `json:"total_token_usage"`
		Last  *codexUsage `json:"last_token_usage"`
	} `json:"info"`
}

// parseCodex reads one rollout; false when its cwd is outside the project. A rollout whose source names a subagent
// is reported as its own session with that subagent's role.
func parseCodex(path string, p project) (Session, bool, error) {
	s := Session{Agent: "codex", ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	var cwd, prompt, subagent string
	var total, previous codexUsage
	responses := 0
	first := int64(-1)
	skills := map[string]int{}
	err := eachLine(path, func(line []byte) {
		var l codexLine
		var pl codexPayload
		if json.Unmarshal(line, &l) != nil || (len(l.Payload) > 0 && json.Unmarshal(l.Payload, &pl) != nil) {
			s.Skipped++
			return
		}
		if s.Start.IsZero() && l.Timestamp != "" {
			s.Start = parseTime(l.Timestamp)
		}
		switch l.Type {
		case "session_meta":
			if pl.ID != "" {
				s.ID = pl.ID
			}
			if pl.Timestamp != "" {
				s.Start = parseTime(pl.Timestamp)
			}
			cwd = pl.CWD
			subagent = codexSubagent(pl.Source)
		case "turn_context":
			if s.Model == "" {
				s.Model = pl.Model
			}
			if cwd == "" {
				cwd = pl.CWD
			}
		case "event_msg":
			switch {
			case pl.Type == "user_message" && prompt == "":
				prompt = jsonString(pl.Message)
			case pl.Type == "token_count" && pl.Info != nil && pl.Info.Total != nil && *pl.Info.Total != previous:
				previous, total = *pl.Info.Total, *pl.Info.Total
				responses++
				if first < 0 && pl.Info.Last != nil {
					first = pl.Info.Last.Input
				}
			}
		case "response_item":
			if pl.Type != "message" && pl.Type != "reasoning" {
				skillReads(skills, l.Payload, skillShellRead)
			}
		}
	})
	if err != nil {
		return Session{}, false, err
	}
	if !p.under(cwd) {
		return Session{}, false, nil
	}
	if first > 0 {
		s.StartContext = first
	}
	name := "main"
	if subagent != "" {
		name = p.roleOf(subagent, prompt)
	}
	rs := roles{}
	rs.add(name, 1, Tokens{Responses: responses, Input: total.Input - total.Cached, CacheRead: total.Cached,
		Output: total.Output}, skills)
	s.Roles = rs.list()
	return s, true, nil
}

// codexSubagent is the subagent kind in a session source such as `{"subagent": "review"}`; empty for a session a
// person started (`"cli"`, `"vscode"`, `"exec"`).
func codexSubagent(source json.RawMessage) string {
	var object map[string]json.RawMessage
	if json.Unmarshal(source, &object) != nil {
		return ""
	}
	raw, ok := object["subagent"]
	if !ok {
		return ""
	}
	if kind := jsonString(raw); kind != "" {
		return kind
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(raw, &nested) == nil && len(nested) > 0 {
		kinds := make([]string, 0, len(nested))
		for kind := range nested {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		return kinds[0]
	}
	return "subagent"
}
