package knowledge

import (
	"strings"
)

// Marker is the verified marker of a proposal accepted on date (yyyy-mm-dd).
func Marker(p Proposal, date string) string {
	return "<!-- verified " + p.Repo + "@" + p.ShortCommit() + " " + date + " -->"
}

// Merge adds the proposal's content, ending with its verified marker, to the end of the page section whose heading
// (level 2 or deeper, case-insensitive) matches the proposal's, before the next heading of the same or a higher
// level; with no such section it appends `## <heading>` at the end of the page. Headings in fenced code are text.
func Merge(page string, p Proposal, date string) string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(page, "\r\n", "\n"), "\n"), "\n")
	content := strings.Split(withMarker(p.Content, Marker(p, date)), "\n")

	start, level := -1, 0
	end := len(lines)
	openFence := ""
	for i, line := range lines {
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
		match := heading.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		switch {
		case start < 0 && len(match[1]) >= 2 && strings.EqualFold(strings.TrimSpace(match[2]), strings.TrimSpace(p.Heading)):
			start, level = i, len(match[1])
		case start >= 0 && len(match[1]) <= level:
			end = i
		}
		if end < len(lines) {
			break
		}
	}

	if start < 0 {
		out := append(trimTrailingBlank(lines), "", "## "+p.Heading, "")
		return strings.Join(append(out, content...), "\n") + "\n"
	}
	before := trimTrailingBlank(lines[:end])
	after := lines[end:]
	out := append(append(append([]string{}, before...), ""), content...)
	if len(after) > 0 {
		out = append(append(out, ""), after...)
	}
	return strings.Join(out, "\n") + "\n"
}

// withMarker ends the content's last line with the marker, or puts it on its own line after a closing fence.
func withMarker(content, marker string) string {
	lines := strings.Split(content, "\n")
	last := lines[len(lines)-1]
	if fence.MatchString(last) {
		return content + "\n" + marker
	}
	return content + " " + marker
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// NewPage is the page template for a proposal's target, reduced to its front matter and headings so no placeholder
// text lands in the page: `Feature name` becomes the title (the target with dashes as spaces and a capital first
// letter, or the repo name itself for a repo page) and `repo-name` the proposal's repo.
func NewPage(template string, p Proposal) string {
	title := p.Target
	if p.Kind == "feature" {
		title = strings.NewReplacer("-", " ", "_", " ").Replace(title)
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	text := strings.NewReplacer("Feature name", title, "repo-name", p.Repo).Replace(strings.ReplaceAll(template, "\r\n", "\n"))
	var out []string
	body := text
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[3:], "\n---\n"); end >= 0 {
			out = append(out, strings.Split(text[:3+end+4], "\n")...)
			body = text[3+end+5:]
		}
	}
	openFence := ""
	for _, line := range strings.Split(body, "\n") {
		if match := fence.FindStringSubmatch(line); match != nil {
			if openFence == "" {
				openFence = match[1]
			} else if openFence == match[1] {
				openFence = ""
			}
			continue
		}
		if openFence == "" && heading.MatchString(line) {
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// PruneEmptySections drops the level 2 and deeper headings that have nothing under them before the next heading of
// the same or a higher level: the template's headings no proposal filled. Front matter, the title and fenced code
// stay as they are.
func PruneEmptySections(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				start = i + 1
				break
			}
		}
	}
	levels := make([]int, len(lines))
	openFence := ""
	for i := start; i < len(lines); i++ {
		if match := fence.FindStringSubmatch(lines[i]); match != nil {
			if openFence == "" {
				openFence = match[1]
			} else if openFence == match[1] {
				openFence = ""
			}
			continue
		}
		if openFence == "" {
			if match := heading.FindStringSubmatch(lines[i]); match != nil {
				levels[i] = len(match[1])
			}
		}
	}
	drop := make([]bool, len(lines))
	for i := len(lines) - 1; i >= start; i-- {
		if levels[i] < 2 {
			continue
		}
		empty := true
		for j := i + 1; j < len(lines); j++ {
			if levels[j] > 0 && levels[j] <= levels[i] {
				break
			}
			if drop[j] || strings.TrimSpace(lines[j]) == "" {
				continue
			}
			empty = false
			break
		}
		if empty {
			for j := i; j < len(lines); j++ {
				if j > i && levels[j] > 0 && levels[j] <= levels[i] {
					break
				}
				drop[j] = true
			}
		}
	}
	var out []string
	for i, line := range lines {
		if drop[i] {
			continue
		}
		if strings.TrimSpace(line) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}
