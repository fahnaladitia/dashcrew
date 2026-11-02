package knowledge

import (
	"errors"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Proposal is one retro lesson in `inbox/`, waiting for review:
//
//	---
//	kind: feature | repo
//	target: <page slug>
//	repo: <repo>
//	branch: <branch>
//	commit: <sha>
//	---
//
//	## <heading>
//
//	<content, with code pointers>
type Proposal struct {
	Kind, Target, Repo, Branch, Commit string
	// Heading is the section the content merges under: the body's heading when the body starts with one, else Notes.
	Heading string
	// Content is the body without that heading, trimmed.
	Content string
}

// NotesHeading is the section a proposal without a heading merges under.
const NotesHeading = "Notes"

var (
	slugPattern   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

var proposalKeys = []string{"kind", "target", "repo", "branch", "commit"}

// PagePath is the page the proposal merges into, slash-separated: `features/<target>.md` or `repos/<target>.md`.
func (p Proposal) PagePath() string {
	if p.Kind == "repo" {
		return "repos/" + p.Target + ".md"
	}
	return "features/" + p.Target + ".md"
}

// ShortCommit is the first 7 characters of the commit, as the verified marker records it.
func (p Proposal) ShortCommit() string { return short(p.Commit) }

// ParseProposal reads a proposal; the error says what does not follow the format.
func ParseProposal(text string) (Proposal, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Proposal{}, errors.New("no front matter; start with --- kind, target, repo, branch, commit ---")
	}
	end := strings.Index(text[3:], "\n---\n")
	if end < 0 {
		return Proposal{}, errors.New("front matter is not closed with ---")
	}
	value, err := yamlutil.Parse(text[4 : 3+end+1])
	if err != nil {
		return Proposal{}, errors.New("front matter is not valid YAML: " + err.Error())
	}
	fields, ok := value.(*omap.Map[any])
	if !ok {
		return Proposal{}, errors.New("front matter is not a map")
	}
	for _, key := range fields.Keys() {
		if !contains(proposalKeys, key) {
			return Proposal{}, errors.New(key + ": unknown key")
		}
	}
	read := func(key string) string {
		switch v := fields.Value(key).(type) {
		case nil, *omap.Map[any], []any:
			return ""
		default:
			return strings.TrimSpace(yamlutil.DartString(v))
		}
	}
	p := Proposal{Kind: read("kind"), Target: read("target"), Repo: read("repo"), Branch: read("branch"), Commit: read("commit")}
	for _, key := range proposalKeys {
		if read(key) == "" {
			return Proposal{}, errors.New(key + ": required")
		}
	}
	if p.Kind != "feature" && p.Kind != "repo" {
		return Proposal{}, errors.New("kind: expected feature or repo")
	}
	slug, slugErr := pageSlug(p.Kind, p.Target)
	if slugErr != nil {
		return Proposal{}, slugErr
	}
	p.Target = slug
	switch {
	case !slugPattern.MatchString(p.Target):
		return Proposal{}, errors.New("target: expected a page slug of letters, digits, '.', '_' or '-'")
	case !slugPattern.MatchString(p.Repo):
		return Proposal{}, errors.New("repo: expected a repo name of letters, digits, '.', '_' or '-'")
	case !commitPattern.MatchString(p.Commit):
		return Proposal{}, errors.New("commit: expected a commit sha of 7 to 40 hex digits")
	}
	p.Heading, p.Content = splitHeading(text[3+end+5:])
	if p.Content == "" {
		return Proposal{}, errors.New("no content below the front matter")
	}
	return p, nil
}

// splitHeading takes the body's heading when it is the first non-blank line; the rest is the content.
func splitHeading(body string) (string, string) {
	body = strings.Trim(body, "\n")
	first, rest, _ := strings.Cut(strings.TrimLeft(body, " \t\n"), "\n")
	if match := heading.FindStringSubmatch(first); match != nil {
		return match[2], trimBlankLines(rest)
	}
	return NotesHeading, trimBlankLines(body)
}

// trimBlankLines drops leading and trailing blank lines and trailing spaces.
func trimBlankLines(text string) string {
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t")
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// pageSlug is target as a slug. A page path (`features/orders.md`, `repos/shop-api.md`) is reduced to its slug when
// its folder matches kind, since agents write it that way; the other folder is an error.
func pageSlug(kind, target string) (string, error) {
	folders := map[string]string{"feature": "features/", "repo": "repos/"}
	for k, folder := range folders {
		rest, ok := strings.CutPrefix(target, folder)
		if !ok {
			continue
		}
		if k != kind {
			return "", errors.New("target: " + target + " is a " + k + " page, but kind is " + kind)
		}
		return strings.TrimSuffix(rest, ".md"), nil
	}
	return target, nil
}
