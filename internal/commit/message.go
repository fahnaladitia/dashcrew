// Package commit builds and checks commit messages in the team's pattern (`project.commit`): `dashcrew commit
// suggest` derives the type, scope and ticket and prints a ready command, `dashcrew commit lint` and `dashcrew check`
// check messages. Nothing here commits; git is only read, apart from objects a temporary index writes.
package commit

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Parts are the pieces of one message; an empty Type or Subject stays a placeholder.
type Parts struct {
	Type, Scope, Ticket, Subject, Body string
}

// segment is a literal run of a pattern, or a placeholder with the brackets that wrap it.
type segment struct {
	literal     string
	name        string
	open, close string
}

var patternToken = regexp.MustCompile(`\(<(scope|ticket)>\)|\[<(scope|ticket)>\]|<(type|scope|subject|ticket)>`)

func segments(pattern string) []segment {
	var out []segment
	last := 0
	for _, m := range patternToken.FindAllStringSubmatchIndex(pattern, -1) {
		if m[0] > last {
			out = append(out, segment{literal: pattern[last:m[0]]})
		}
		switch {
		case m[2] >= 0:
			out = append(out, segment{name: pattern[m[2]:m[3]], open: "(", close: ")"})
		case m[4] >= 0:
			out = append(out, segment{name: pattern[m[4]:m[5]], open: "[", close: "]"})
		default:
			out = append(out, segment{name: pattern[m[6]:m[7]]})
		}
		last = m[1]
	}
	if last < len(pattern) {
		out = append(out, segment{literal: pattern[last:]})
	}
	return out
}

func optional(name string) bool { return name == "scope" || name == "ticket" }

// dropSpace removes the one space next to the placeholder at i: the following one, else the preceding one.
// It reports which side it took, so Header and the lint pattern drop the same space.
func dropSpace(segs []segment, i int) (next bool, prev bool) {
	if i+1 < len(segs) && strings.HasPrefix(segs[i+1].literal, " ") {
		segs[i+1].literal = segs[i+1].literal[1:]
		return true, false
	}
	if i > 0 && strings.HasSuffix(segs[i-1].literal, " ") {
		segs[i-1].literal = strings.TrimSuffix(segs[i-1].literal, " ")
		return false, true
	}
	return false, false
}

// Header fills the pattern. An empty scope or ticket is dropped with its brackets and one space next to it.
func Header(cfg *profile.Commit, p Parts) string {
	values := map[string]string{"type": p.Type, "scope": p.Scope, "ticket": p.Ticket, "subject": p.Subject}
	segs := segments(cfg.Pattern)
	for i, s := range segs {
		if s.name != "" && optional(s.name) && values[s.name] == "" {
			dropSpace(segs, i)
			segs[i].name = "-"
		}
	}
	var b strings.Builder
	for _, s := range segs {
		switch {
		case s.name == "-":
		case s.name == "":
			b.WriteString(s.literal)
		case values[s.name] == "":
			b.WriteString("<" + s.name + ">")
		default:
			b.WriteString(s.open + values[s.name] + s.close)
		}
	}
	return b.String()
}

// TrailerLines are the trailers suggest adds: the ticket trailer when the profile names one and a ticket is known.
func TrailerLines(cfg *profile.Commit, p Parts) []string {
	if cfg.Ticket == nil || cfg.Ticket.Trailer == "" || p.Ticket == "" {
		return nil
	}
	return []string{cfg.Ticket.Trailer + ": " + p.Ticket}
}

// Message is the header, the body and the trailers, separated by blank lines.
func Message(cfg *profile.Commit, p Parts) string {
	paragraphs := []string{Header(cfg, p)}
	if body := strings.TrimSpace(p.Body); body != "" {
		paragraphs = append(paragraphs, body)
	}
	if trailers := TrailerLines(cfg, p); len(trailers) > 0 {
		paragraphs = append(paragraphs, strings.Join(trailers, "\n"))
	}
	return strings.Join(paragraphs, "\n\n")
}

// headerPattern matches a filled header; `!` before a colon after the type or scope marks a breaking change.
func headerPattern(pattern string) *regexp.Regexp {
	segs := segments(pattern)
	parts := make([]string, len(segs))
	for i, s := range segs {
		if s.name == "" {
			continue
		}
		var inner string
		switch {
		case s.name == "type":
			inner = `(?P<type>[^\s(\[!:]+)`
		case s.name == "subject":
			inner = `(?P<subject>\S(?:.*\S)?)`
		case s.open != "":
			inner = regexp.QuoteMeta(s.open) + `(?P<` + s.name + `>[^()\[\]\s][^()\[\]]*)` + regexp.QuoteMeta(s.close)
		default:
			inner = `(?P<` + s.name + `>\S+)`
		}
		if !optional(s.name) {
			parts[i] = inner
			continue
		}
		next, prev := dropSpace(segs, i)
		switch {
		case next:
			parts[i] = "(?:" + inner + " )?"
		case prev:
			parts[i] = "(?: " + inner + ")?"
		default:
			parts[i] = "(?:" + inner + ")?"
		}
	}
	var b strings.Builder
	b.WriteString("^")
	for i, s := range segs {
		if s.name != "" {
			b.WriteString(parts[i])
			continue
		}
		if strings.HasPrefix(s.literal, ":") && i > 0 && (segs[i-1].name == "type" || segs[i-1].open != "") {
			b.WriteString("!?")
		}
		b.WriteString(regexp.QuoteMeta(s.literal))
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

var (
	trailerLine     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*): \S`)
	placeholderLeft = regexp.MustCompile(`<(type|scope|subject|ticket|body)>`)
	aiName          = `(?:claude|anthropic|copilot|chatgpt|openai|gpt|codex|gemini|cursor|devin|aider|ai)`
	generatedWith   = regexp.MustCompile(`(?i)\bgenerated (?:with|by|using)\b.*\b` + aiName + `\b`)
	aiCoAuthor      = regexp.MustCompile(`(?i)^co-authored-by:.*\b` + aiName + `\b`)
	// emailAddress is `local@domain.tld` with a letters-only top-level label.
	emailAddress = regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,})\b`)
	// scaleSuffix is an image density suffix (`icon@2x.png`, `a@1.5x.webp`): a file name, not an address.
	scaleSuffix = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?x\.`)
	// BaseTrailers are allowed in every profile, before the ticket trailer and `project.commit.trailers`.
	BaseTrailers = []string{"Co-authored-by", "Signed-off-by", "Reviewed-by"}
)

// Lint returns one line per problem of message against the profile: the pattern, the type, the subject line
// length, the body rule, the trailers, email addresses and AI attribution. Nil means the message passes.
func Lint(cfg *profile.Commit, message string) []string {
	text := strings.TrimRight(strings.ReplaceAll(message, "\r\n", "\n"), " \t\n")
	if strings.TrimSpace(text) == "" {
		return []string{"message is empty"}
	}
	lines := strings.Split(text, "\n")
	var problems []string
	header := lines[0]
	if match := headerPattern(cfg.Pattern).FindStringSubmatch(header); match == nil {
		problems = append(problems, fmt.Sprintf("subject line %q does not match %s", header, cfg.Pattern))
	} else if index := headerPattern(cfg.Pattern).SubexpIndex("type"); index >= 0 && !placeholderLeft.MatchString(match[index]) {
		if !contains(cfg.Types, match[index]) {
			problems = append(problems, fmt.Sprintf("type %q is not one of %s", match[index], strings.Join(cfg.Types, ", ")))
		}
	}
	if n := utf8.RuneCountInString(header); n > cfg.SubjectMax {
		problems = append(problems, fmt.Sprintf("subject line is %d characters, over %d", n, cfg.SubjectMax))
	}
	seen := map[string]bool{}
	for _, m := range placeholderLeft.FindAllString(text, -1) {
		if !seen[m] {
			seen[m] = true
			problems = append(problems, "placeholder "+m+" is not filled")
		}
	}
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		problems = append(problems, "line 2 must be blank, between the subject line and the body")
	}

	body, trailers := splitBody(lines[1:])
	switch {
	case cfg.Body == "required" && body == "":
		problems = append(problems, "body is required")
	case cfg.Body == "none" && body != "":
		problems = append(problems, "body is not allowed")
	}
	allowed := append([]string{}, BaseTrailers...)
	if cfg.Ticket != nil && cfg.Ticket.Trailer != "" {
		allowed = append(allowed, cfg.Ticket.Trailer)
	}
	allowed = append(allowed, cfg.Trailers...)
	for _, line := range trailers {
		key := trailerLine.FindStringSubmatch(line)[1]
		if !containsFold(allowed, key) {
			problems = append(problems, fmt.Sprintf("trailer %q is not allowed; allowed: %s", key, strings.Join(allowed, ", ")))
		}
	}
	for i, line := range lines {
		if HasEmail(line) && !(contains(trailers, line) && containsFold(cfg.AllowEmailTrailers, trailerLine.FindStringSubmatch(line)[1])) {
			problems = append(problems, fmt.Sprintf("email address on line %d: commit messages never show one", i+1))
		}
	}
	for i, line := range lines {
		switch {
		case generatedWith.MatchString(line):
			problems = append(problems, fmt.Sprintf("AI attribution: generated-with footer on line %d", i+1))
		case aiCoAuthor.MatchString(strings.TrimSpace(line)):
			problems = append(problems, fmt.Sprintf("AI attribution: AI co-author trailer on line %d", i+1))
		case strings.Contains(line, "\U0001F916"):
			problems = append(problems, fmt.Sprintf("AI attribution: robot line on line %d", i+1))
		}
	}
	return problems
}

// emails are the index pairs of the email addresses in text; `@mention`, a lone `name@` and an image density
// suffix such as `icon@2x.png` are not addresses.
func emails(text string) [][]int {
	var out [][]int
	for _, m := range emailAddress.FindAllStringSubmatchIndex(text, -1) {
		if !scaleSuffix.MatchString(text[m[2]:m[3]]) {
			out = append(out, m[:2])
		}
	}
	return out
}

// HasEmail is true when text holds an email address.
func HasEmail(text string) bool { return len(emails(text)) > 0 }

// RedactEmails replaces every email address in text with `<email>`.
func RedactEmails(text string) string {
	var b strings.Builder
	last := 0
	for _, m := range emails(text) {
		b.WriteString(text[last:m[0]] + "<email>")
		last = m[1]
	}
	return b.String() + text[last:]
}

// splitBody parts the lines after the subject line into the body and the trailer block: the last paragraph, when
// every line of it is `Key: value`.
func splitBody(rest []string) (string, []string) {
	var paragraphs [][]string
	var current []string
	for _, line := range rest {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, current)
				current = nil
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	var trailers []string
	if n := len(paragraphs); n > 0 {
		last := paragraphs[n-1]
		isTrailers := true
		for _, line := range last {
			isTrailers = isTrailers && trailerLine.MatchString(line)
		}
		if isTrailers {
			trailers = last
			paragraphs = paragraphs[:n-1]
		}
	}
	var body []string
	for _, paragraph := range paragraphs {
		body = append(body, strings.Join(paragraph, "\n"))
	}
	return strings.Join(body, "\n\n"), trailers
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}
