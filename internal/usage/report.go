package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Text writes one block per session, then a total line. Times are shown in loc.
func Text(w io.Writer, sessions []Session, loc *time.Location) {
	var total Tokens
	for _, s := range sessions {
		head := []string{s.Agent, short(s.ID), s.Start.In(loc).Format("2006-01-02 15:04")}
		if s.Model != "" {
			head = append(head, s.Model)
		}
		head = append(head, "start "+amount(s.StartContext))
		if s.Skipped > 0 {
			head = append(head, fmt.Sprintf("skipped %d lines", s.Skipped))
		}
		fmt.Fprintln(w, strings.Join(head, "  "))

		rows := [][]string{{"role", "runs", "responses", "input", "cache-write", "cache-read", "output"}}
		var skills []string
		for _, r := range s.Roles {
			total.add(r.Tokens)
			rows = append(rows, []string{r.Name, strconv.Itoa(r.Runs), strconv.Itoa(r.Responses), amount(r.Input),
				amount(r.CacheWrite), amount(r.CacheRead), amount(r.Output)})
			if len(r.Skills) > 0 {
				skills = append(skills, r.Name+": "+skillList(r.Skills))
			}
		}
		widths := make([]int, len(rows[0]))
		for _, row := range rows {
			for i, cell := range row {
				widths[i] = max(widths[i], len(cell))
			}
		}
		for _, row := range rows {
			cells := make([]string, len(row))
			for i, cell := range row {
				if i == 0 {
					cells[i] = cell + strings.Repeat(" ", widths[i]-len(cell))
				} else {
					cells[i] = strings.Repeat(" ", widths[i]-len(cell)) + cell
				}
			}
			fmt.Fprintln(w, "  "+strings.Join(cells, "  "))
		}
		if len(skills) > 0 {
			fmt.Fprintln(w, "  "+"skills"+strings.Repeat(" ", max(0, widths[0]-len("skills")))+"  "+strings.Join(skills, "; "))
		}
	}
	noun := "sessions"
	if len(sessions) == 1 {
		noun = "session"
	}
	fmt.Fprintf(w, "total  %d %s  input %s  cache-write %s  cache-read %s  output %s\n", len(sessions), noun,
		amount(total.Input), amount(total.CacheWrite), amount(total.CacheRead), amount(total.Output))
}

// short is the first 8 characters of a session id.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// amount is n below 1000, else n in k or M with one decimal.
func amount(n int64) string {
	switch {
	case n < 1000:
		return strconv.FormatInt(n, 10)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

// skillList is the skills by name, with ` xN` when loaded more than once.
func skillList(skills map[string]int) string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		if n := skills[name]; n > 1 {
			names[i] = fmt.Sprintf("%s x%d", name, n)
		}
	}
	return strings.Join(names, ", ")
}

type jsonRole struct {
	Role       string         `json:"role"`
	Runs       int            `json:"runs"`
	Responses  int            `json:"responses"`
	Input      int64          `json:"input"`
	CacheWrite int64          `json:"cache_write"`
	CacheRead  int64          `json:"cache_read"`
	Output     int64          `json:"output"`
	Skills     map[string]int `json:"skills,omitempty"`
}

type jsonSession struct {
	Agent        string     `json:"agent"`
	ID           string     `json:"id"`
	Start        string     `json:"start"`
	Model        string     `json:"model,omitempty"`
	StartContext int64      `json:"start_context"`
	SkippedLines int        `json:"skipped_lines"`
	Roles        []jsonRole `json:"roles"`
}

// JSON writes the sessions as one indented JSON array, numbers unformatted and times in UTC.
func JSON(w io.Writer, sessions []Session) error {
	out := make([]jsonSession, 0, len(sessions))
	for _, s := range sessions {
		js := jsonSession{Agent: s.Agent, ID: s.ID, Start: s.Start.UTC().Format(time.RFC3339), Model: s.Model,
			StartContext: s.StartContext, SkippedLines: s.Skipped, Roles: []jsonRole{}}
		for _, r := range s.Roles {
			js.Roles = append(js.Roles, jsonRole{Role: r.Name, Runs: r.Runs, Responses: r.Responses, Input: r.Input,
				CacheWrite: r.CacheWrite, CacheRead: r.CacheRead, Output: r.Output, Skills: r.Skills})
		}
		out = append(out, js)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
