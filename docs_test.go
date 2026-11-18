package dashcrew

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// userDocs are the Markdown files a visitor reads; docs/plans and docs/ideas are working notes and are not checked.
func userDocs(t *testing.T) []string {
	t.Helper()
	files := []string{"README.md", "CONTRIBUTING.md", "SECURITY.md", "CHANGELOG.md"}
	more, err := filepath.Glob("docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	return append(files, more...)
}

func readDoc(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var (
	fence     = regexp.MustCompile("(?ms)^```.*?^```")
	codeSpan  = regexp.MustCompile("`[^`\n]*`")
	mdLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	heading   = regexp.MustCompile(`(?m)^#{1,6} +(.+?) *$`)
	planCode  = regexp.MustCompile(`\bP[0-9]+[a-z]?\b`)
	slugStrip = regexp.MustCompile(`[^\p{L}\p{N} _-]`)
)

// prose is the text of a Markdown file without fenced blocks and code spans.
func prose(text string) string {
	return codeSpan.ReplaceAllString(fence.ReplaceAllString(text, ""), "")
}

// anchors are the heading anchors GitHub gives a file: lower case, punctuation dropped, spaces to "-", and "-1",
// "-2" on repeats.
func anchors(text string) map[string]bool {
	found := map[string]bool{}
	seen := map[string]int{}
	for _, m := range heading.FindAllStringSubmatch(fence.ReplaceAllString(text, ""), -1) {
		slug := strings.ReplaceAll(slugStrip.ReplaceAllString(strings.ToLower(m[1]), ""), " ", "-")
		if n := seen[slug]; n > 0 {
			found[slug+"-"+strconv.Itoa(n)] = true
		} else {
			found[slug] = true
		}
		seen[slug]++
	}
	return found
}

func TestReadmeIsShort(t *testing.T) {
	if n := strings.Count(readDoc(t, "README.md"), "\n"); n > 150 {
		t.Errorf("README.md has %d lines; keep it under 150 and move reference material to docs/", n)
	}
}

func TestDocLinksResolve(t *testing.T) {
	for _, name := range userDocs(t) {
		text := readDoc(t, name)
		for _, m := range mdLink.FindAllStringSubmatch(prose(text), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file, anchor, _ := strings.Cut(target, "#")
			path := name
			if file != "" {
				path = filepath.Join(filepath.Dir(name), file)
				if _, err := os.Stat(path); err != nil {
					t.Errorf("%s links to %s, which does not exist", name, target)
					continue
				}
			}
			if anchor == "" || !strings.HasSuffix(path, ".md") {
				continue
			}
			if !anchors(readDoc(t, path))[anchor] {
				t.Errorf("%s links to %s, but %s has no heading with that anchor", name, target, path)
			}
		}
	}
}

// maintainerDocs name plans by their code on purpose: the roadmap tracks them, and the checklist states the rule.
var maintainerDocs = map[string]bool{"docs/ROADMAP.md": true, "docs/release-checklist.md": true}

func TestDocsHaveNoPlanCodes(t *testing.T) {
	for _, name := range userDocs(t) {
		if maintainerDocs[filepath.ToSlash(name)] {
			continue
		}
		for _, code := range planCode.FindAllString(prose(readDoc(t, name)), -1) {
			t.Errorf("%s names the internal plan code %s; say what changed instead", name, code)
		}
	}
}

func TestAnchors(t *testing.T) {
	got := anchors("# dashcrew\n## `dashcrew me`\n### `dashcrew guard [<policy>]`\n## Install\n## Install\n```\n# not a heading\n```\n")
	for _, want := range []string{"dashcrew", "dashcrew-me", "dashcrew-guard-policy", "install", "install-1"} {
		if !got[want] {
			t.Errorf("anchors lack %q: %v", want, got)
		}
	}
	if got["not-a-heading"] {
		t.Error("a line in a fenced block was read as a heading")
	}
}
