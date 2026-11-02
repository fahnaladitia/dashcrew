// Package knowledge is the shared knowledge repo: its page format, the scaffold of a new repo, and the local
// clone each member keeps under `~/.dashcrew/knowledge/<name>`, which sync links into a project as `.ai/knowledge`.
//
// A page is Markdown with front matter (`title`, `repos`, `tags`). Every fact paragraph ends with a verified
// marker, `<!-- verified <repo>@<sha> <yyyy-mm-dd> -->`, and code pointers are written `<repo>:<path>:<line>` in
// backticks so tools can find them.
package knowledge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Page is one parsed knowledge page. Line numbers count from 1 over the whole file, front matter included.
type Page struct {
	// FrontMatter is true when the page starts with a closed `---` block.
	FrontMatter bool
	// Title is the front matter title, else the first level 1 heading.
	Title    string
	Repos    []string
	Tags     []string
	Headings []Heading
	Verified []Verified
	Pointers []Pointer
	// Problems are what does not follow the format; a page with problems is still parsed as far as it goes.
	Problems []Problem
	// Body is the text after the front matter, with line endings as `\n`.
	Body string
}

// Heading is an ATX heading outside fenced code.
type Heading struct {
	Level int
	Text  string
	Line  int
}

// Verified is one `<!-- verified <repo>@<sha> <yyyy-mm-dd> -->` marker.
type Verified struct {
	Repo, SHA, Date string
	Line            int
}

// Pointer is one `<repo>:<path>:<line>` code pointer; CodeLine is the line in the repo, Line the line in the page.
type Pointer struct {
	Repo, Path string
	CodeLine   int
	Line       int
}

func (p Pointer) String() string { return p.Repo + ":" + p.Path + ":" + strconv.Itoa(p.CodeLine) }

// Problem is one place where a page does not follow the format.
type Problem struct {
	Line    int
	Message string
}

func (p Problem) String() string { return "line " + strconv.Itoa(p.Line) + ": " + p.Message }

const markerShape = "<!-- verified <repo>@<sha> <yyyy-mm-dd> -->"

var (
	namePattern  = `[A-Za-z0-9._-]+`
	markerStart  = regexp.MustCompile(`<!--\s*verified\b`)
	markerStrict = regexp.MustCompile(`^<!-- verified (` + namePattern + `)@([0-9a-f]{7,40}) ([0-9]{4}-[0-9]{2}-[0-9]{2}) -->$`)
	pointer      = regexp.MustCompile("`(" + namePattern + "):([^\\s:`]+):([1-9][0-9]*)`")
	heading      = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)
	fence        = regexp.MustCompile("^ {0,3}(```|~~~)")
)

// Parse reads a page. It never fails: what does not follow the format is listed in Problems.
func Parse(text string) Page {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	page := Page{Body: text}
	bodyStart := 1
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[3:], "\n---\n"); end >= 0 || strings.HasSuffix(text, "\n---") {
			var yaml string
			if end >= 0 {
				yaml = text[4 : 3+end+1]
				page.Body = text[3+end+5:]
			} else {
				yaml = text[4 : len(text)-3]
				page.Body = ""
			}
			page.FrontMatter = true
			bodyStart = strings.Count(yaml, "\n") + 3
			page.readFrontMatter(yaml)
		} else {
			page.problem(1, "front matter is not closed with ---")
		}
	} else {
		page.problem(1, "no front matter; start the page with --- title, repos, tags ---")
	}
	page.readBody(bodyStart)
	if page.Title == "" {
		for _, h := range page.Headings {
			if h.Level == 1 {
				page.Title = h.Text
				break
			}
		}
	}
	return page
}

func (p *Page) problem(line int, message string) {
	p.Problems = append(p.Problems, Problem{line, message})
}

func (p *Page) readFrontMatter(yaml string) {
	value, err := yamlutil.Parse(yaml)
	if err != nil {
		p.problem(1, "front matter is not valid YAML: "+err.Error())
		return
	}
	if value == nil {
		value = omap.New[any]()
	}
	fields, ok := value.(*omap.Map[any])
	if !ok {
		p.problem(1, "front matter is not a map")
		return
	}
	switch title := fields.Value("title").(type) {
	case nil:
		p.problem(1, "front matter has no title")
	case string:
		p.Title = strings.TrimSpace(title)
	default:
		p.problem(1, "front matter title: expected a string")
	}
	for _, key := range []string{"repos", "tags"} {
		value := fields.Value(key)
		if value == nil {
			continue
		}
		list, ok := names(value)
		if !ok {
			p.problem(1, "front matter "+key+": expected a list of names")
			continue
		}
		if key == "repos" {
			p.Repos = list
		} else {
			p.Tags = list
		}
	}
}

// names is a YAML list of non-empty strings.
func names(value any) ([]string, bool) {
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

// readBody reads headings, markers and pointers outside fenced code; first is the file line of the body's first line.
func (p *Page) readBody(first int) {
	openFence := ""
	for i, line := range strings.Split(p.Body, "\n") {
		number := first + i
		if match := fence.FindStringSubmatch(line); match != nil {
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
		if match := heading.FindStringSubmatch(line); match != nil {
			p.Headings = append(p.Headings, Heading{len(match[1]), match[2], number})
		}
		p.readMarkers(line, number)
		for _, match := range pointer.FindAllStringSubmatch(line, -1) {
			codeLine, _ := strconv.Atoi(match[3])
			p.Pointers = append(p.Pointers, Pointer{Repo: match[1], Path: match[2], CodeLine: codeLine, Line: number})
		}
	}
}

func (p *Page) readMarkers(line string, number int) {
	for _, start := range markerStart.FindAllStringIndex(line, -1) {
		rest := line[start[0]:]
		end := strings.Index(rest, "-->")
		if end < 0 {
			p.problem(number, "verified marker is not closed with -->")
			continue
		}
		match := markerStrict.FindStringSubmatch(rest[:end+3])
		if match == nil {
			p.problem(number, "malformed verified marker; want "+markerShape)
			continue
		}
		if _, err := time.Parse("2006-01-02", match[3]); err != nil {
			p.problem(number, fmt.Sprintf("verified marker date %s is not a calendar date", match[3]))
			continue
		}
		p.Verified = append(p.Verified, Verified{Repo: match[1], SHA: match[2], Date: match[3], Line: number})
	}
}
