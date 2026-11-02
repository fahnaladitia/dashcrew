// Package spec is the format of a feature spec in the knowledge repo, `specs/<release>/<feature>/prd.md` and
// `trd.md`: front matter, the IDs `AC-n`, `D-n` and `C-n`, and open questions; its lint, scaffold and revise.
//
// An ID is written at the start of a list item or heading and is never renumbered. A dropped one stays, struck
// through with its reason: `~~AC-3~~ dropped: <why>`.
package spec

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/commit"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Dir is the knowledge repo folder that holds specs.
const Dir = "specs"

// Statuses of a spec file.
const (
	Draft      = "draft"
	Review     = "review"
	Approved   = "approved"
	Superseded = "superseded"
)

// Statuses lists every status in its order of life.
var Statuses = []string{Draft, Review, Approved, Superseded}

// Files are the two files of a spec, in the order commands report them.
var Files = []string{"prd.md", "trd.md"}

// Writable is true for a status agents may still write: draft and review.
func Writable(status string) bool { return status == Draft || status == Review }

// requiredKeys are the front matter keys every spec file holds, in the order problems name them.
var requiredKeys = []string{"title", "feature", "release", "status", "rev", "repos", "owners"}

// Doc is one parsed spec file. Line numbers count from 1 over the whole file, front matter included.
type Doc struct {
	FrontMatter                     bool
	Title, Feature, Release, Status string
	Rev                             int
	// RevLine is the line of `rev:`, where a rev problem is reported; 1 when it is missing.
	RevLine       int
	Repos, Owners []string
	IDs           []ID
	Questions     []Question
	// Problems are what does not follow the format; a file with problems is still parsed as far as it goes.
	Problems []Problem
	// text is the parsed text with line endings as `\n`.
	text string
}

// ID is one `AC-n`, `D-n` or `C-n` at the start of a list item or heading.
type ID struct {
	Kind string
	N    int
	// Text is what follows the ID, spaces collapsed; for a dropped ID, the text inside the strike.
	Text    string
	Dropped bool
	Reason  string
	Line    int
}

func (id ID) String() string { return id.Kind + "-" + strconv.Itoa(id.N) }

// Question is one open question; Owner is empty when the item names none.
type Question struct {
	Text, Owner string
	Line        int
}

// Problem is one place where a spec does not follow the format or the rules; Line 0 means the file as a whole.
type Problem struct {
	Line    int
	Message string
}

func (p Problem) String() string {
	if p.Line == 0 {
		return p.Message
	}
	return "line " + strconv.Itoa(p.Line) + ": " + p.Message
}

var (
	listOrHeading = regexp.MustCompile(`^ {0,3}(?:[-*+]|[0-9]+[.)]|#{1,6})[ \t]+(.*)$`)
	listItem      = regexp.MustCompile(`^ {0,3}(?:[-*+]|[0-9]+[.)])[ \t]+(.*)$`)
	headingLine   = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t#]*$`)
	liveID        = regexp.MustCompile(`^(AC|D|C)-([1-9][0-9]*)\b(.*)$`)
	droppedID     = regexp.MustCompile(`^~~(AC|D|C)-([1-9][0-9]*)\b([^~]*)~~(.*)$`)
	dropReason    = regexp.MustCompile(`(?i)^dropped:\s*(\S.*)$`)
	ownerField    = regexp.MustCompile(`(?i)\bowner:\s*([^\s,;)]+)`)
	fenceLine     = regexp.MustCompile("^ {0,3}(```|~~~)")
	spaces        = regexp.MustCompile(`\s+`)
)

// Parse reads a spec file. It never fails: what does not follow the format is listed in Problems.
func Parse(text string) Doc {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	doc := Doc{RevLine: 1, text: text}
	body, bodyStart := text, 1
	switch {
	case !strings.HasPrefix(text, "---\n"):
		doc.problem(1, "no front matter; start the file with --- "+strings.Join(requiredKeys, ", ")+" ---")
	default:
		end := strings.Index(text[3:], "\n---\n")
		if end < 0 {
			doc.problem(1, "front matter is not closed with ---")
			break
		}
		yaml := text[4 : 3+end+1]
		body = text[3+end+5:]
		bodyStart = strings.Count(yaml, "\n") + 3
		doc.FrontMatter = true
		doc.readFrontMatter(yaml)
	}
	doc.readBody(body, bodyStart)
	return doc
}

func (d *Doc) problem(line int, message string) {
	d.Problems = append(d.Problems, Problem{line, message})
}

func (d *Doc) readFrontMatter(yaml string) {
	for i, line := range strings.Split(yaml, "\n") {
		if strings.HasPrefix(line, "rev:") {
			d.RevLine = i + 2
		}
	}
	value, err := yamlutil.Parse(yaml)
	if err != nil {
		d.problem(1, "front matter is not valid YAML: "+err.Error())
		return
	}
	fields, ok := value.(*omap.Map[any])
	if value == nil {
		fields, ok = omap.New[any](), true
	}
	if !ok {
		d.problem(1, "front matter is not a map")
		return
	}
	for _, key := range requiredKeys {
		if !fields.Has(key) {
			d.problem(1, "front matter has no "+key)
		}
	}
	text := func(key string) string {
		switch value := fields.Value(key).(type) {
		case nil:
		case string:
			return strings.TrimSpace(value)
		case int, int64, float64:
			d.problem(1, fmt.Sprintf("front matter %s %s is a number; quote it", key, yamlutil.DartString(value)))
		default:
			d.problem(1, "front matter "+key+": expected text")
		}
		return ""
	}
	d.Title, d.Feature, d.Release, d.Status = text("title"), text("feature"), text("release"), text("status")
	if fields.Has("status") && d.Status != "" && !contains(Statuses, d.Status) {
		d.problem(1, "status "+d.Status+" is not one of "+strings.Join(Statuses, ", "))
	}
	if fields.Has("rev") {
		if rev, ok := fields.Value("rev").(int); ok && rev >= 1 {
			d.Rev = rev
		} else {
			d.problem(d.RevLine, "rev must be a whole number from 1")
		}
	}
	for _, key := range []string{"repos", "owners"} {
		if !fields.Has(key) {
			continue
		}
		list, ok := names(fields.Value(key))
		if !ok {
			d.problem(1, "front matter "+key+": expected a list of names")
			continue
		}
		if key == "repos" {
			d.Repos = list
			continue
		}
		d.Owners = list
		for _, owner := range list {
			if commit.HasEmail(owner) {
				d.problem(1, "owners hold an email address; owners are roles, never people")
				break
			}
		}
	}
}

// names is a YAML list of non-empty strings; null is an empty list.
func names(value any) ([]string, bool) {
	if value == nil {
		return []string{}, true
	}
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func collapse(text string) string { return strings.TrimSpace(spaces.ReplaceAllString(text, " ")) }

// readBody reads IDs and the open questions outside fenced code; first is the file line of the body's first line.
func (d *Doc) readBody(body string, first int) {
	openFence := ""
	questionLevel := 0
	var question *Question
	flush := func() {
		if question != nil {
			if match := ownerField.FindStringSubmatch(question.Text); match != nil {
				question.Owner = match[1]
				if commit.HasEmail(question.Owner) {
					d.problem(question.Line, "open question owner is an email address; owners are roles, never people")
				}
			}
			d.Questions = append(d.Questions, *question)
			question = nil
		}
	}
	for i, line := range strings.Split(body, "\n") {
		number := first + i
		if match := fenceLine.FindStringSubmatch(line); match != nil {
			switch {
			case openFence == "":
				openFence = match[1]
			case openFence == match[1]:
				openFence = ""
			}
			continue
		}
		if openFence != "" {
			continue
		}
		if match := headingLine.FindStringSubmatch(line); match != nil {
			flush()
			level := len(match[1])
			if questionLevel > 0 && level <= questionLevel {
				questionLevel = 0
			}
			if strings.EqualFold(strings.TrimRight(match[2], ":"), "open questions") {
				questionLevel = level
			}
		}
		if match := listOrHeading.FindStringSubmatch(line); match != nil {
			d.readID(strings.TrimSpace(match[1]), number)
		}
		if questionLevel == 0 {
			continue
		}
		if match := listItem.FindStringSubmatch(line); match != nil {
			flush()
			item := strings.TrimSpace(match[1])
			if !strings.HasPrefix(item, "~~") && !strings.EqualFold(strings.TrimRight(item, "."), "none") {
				question = &Question{Text: item, Line: number}
			}
		} else if question != nil && strings.TrimSpace(line) != "" && (line[0] == ' ' || line[0] == '\t') {
			question.Text += " " + strings.TrimSpace(line)
		} else if strings.TrimSpace(line) == "" {
			flush()
		}
	}
	flush()
}

func (d *Doc) readID(rest string, line int) {
	if match := droppedID.FindStringSubmatch(rest); match != nil {
		n, _ := strconv.Atoi(match[2])
		id := ID{Kind: match[1], N: n, Text: collapse(match[3]), Dropped: true, Line: line}
		if reason := dropReason.FindStringSubmatch(strings.TrimSpace(match[4])); reason != nil {
			id.Reason = reason[1]
		} else {
			d.problem(line, fmt.Sprintf("%s is struck through without a reason; write `~~%s~~ dropped: <why>`", id, id))
		}
		d.IDs = append(d.IDs, id)
		return
	}
	if match := liveID.FindStringSubmatch(rest); match != nil {
		n, _ := strconv.Atoi(match[2])
		text := strings.TrimLeft(match[3], " \t:.-)")
		d.IDs = append(d.IDs, ID{Kind: match[1], N: n, Text: collapse(text), Line: line})
	}
}

// statusLimit is how much of a file StatusOf reads: the guard reads no more to answer quickly.
const statusLimit = 2048

// StatusOf is the front matter status within the first 2 KB of a file's content; empty when there is none there.
// It reads lines, not YAML, so the guard stays fast.
func StatusOf(data []byte) string {
	if len(data) > statusLimit {
		data = data[:statusLimit]
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines[1:] {
		line = strings.TrimSuffix(line, "\r")
		if line == "---" || (i == len(lines)-2 && len(data) == statusLimit) {
			return ""
		}
		value, ok := strings.CutPrefix(line, "status:")
		if !ok {
			continue
		}
		if hash := strings.Index(value, " #"); hash >= 0 {
			value = value[:hash]
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		return value
	}
	return ""
}
