package knowledge

import (
	"fmt"
	"strings"
	"testing"
)

const shareProposal = `---
kind: feature
target: share
repo: notes-api
branch: feat/share-expiry
commit: 1a2b3c4d5e6f
---

## Link expiry

Share links expire after 7 days; the API refuses older tokens. ` + "`notes-api:src/share/token.go:42`" + `
`

func TestParseProposalReadsFrontMatterHeadingAndContent(t *testing.T) {
	p, err := ParseProposal(shareProposal)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "feature" || p.Target != "share" || p.Repo != "notes-api" || p.Branch != "feat/share-expiry" || p.Commit != "1a2b3c4d5e6f" {
		t.Errorf("%+v", p)
	}
	if p.PagePath() != "features/share.md" || p.Heading != "Link expiry" {
		t.Errorf("page %s, heading %q", p.PagePath(), p.Heading)
	}
	if p.Content != "Share links expire after 7 days; the API refuses older tokens. `notes-api:src/share/token.go:42`" {
		t.Errorf("content %q", p.Content)
	}
}

func TestParseProposalWithoutAHeadingGoesToNotes(t *testing.T) {
	p, err := ParseProposal("---\nkind: repo\ntarget: notes-api\nrepo: notes-api\nbranch: main\ncommit: '1234567'\n---\nRun the API with make dev.\n\n## Later\n")
	if err != nil {
		t.Fatal(err)
	}
	if p.PagePath() != "repos/notes-api.md" || p.Heading != "Notes" || p.Content != "Run the API with make dev.\n\n## Later" {
		t.Errorf("%+v", p)
	}
}

func TestParseProposalRejectsMalformedProposals(t *testing.T) {
	valid := "kind: feature\ntarget: share\nrepo: notes-api\nbranch: main\ncommit: abcdef1\n"
	for text, want := range map[string]string{
		"just text\n":                                "no front matter",
		"---\nkind: feature\n":                       "front matter is not closed",
		"---\n" + valid + "---\n\n   \n":             "no content",
		"---\n" + valid + "---\n## Only a heading\n": "no content",
		"---\n" + strings.Replace(valid, "feature", "flow", 1) + "---\nx\n":              "kind: expected feature or repo",
		"---\n" + strings.Replace(valid, "target: share\n", "", 1) + "---\nx\n":          "target: required",
		"---\n" + strings.Replace(valid, "share", "../share", 1) + "---\nx\n":            "target: expected a page slug",
		"---\n" + strings.Replace(valid, "repo: notes-api", "repo: a b", 1) + "---\nx\n": "repo: expected a repo name",
		"---\n" + strings.Replace(valid, "branch: main\n", "", 1) + "---\nx\n":           "branch: required",
		"---\n" + strings.Replace(valid, "abcdef1", "xyz", 1) + "---\nx\n":               "commit: expected a commit sha",
		"---\n" + valid + "extra: 1\n---\nx\n":                                           "extra: unknown key",
		"---\n- a\n---\nx\n":                                                             "front matter is not a map",
		"---\nkind: [\n---\nx\n":                                                         "front matter is not valid YAML",
	} {
		_, err := ParseProposal(text)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", text, err, want)
		}
	}
}

func TestProposalTargetAsAPagePath(t *testing.T) {
	const front = "---\nkind: %s\ntarget: %s\nrepo: shop-api\nbranch: main\ncommit: abcdef1\n---\n\n## Cancel\n\nAtomic.\n"
	for _, c := range []struct{ kind, target, want, err string }{
		{"feature", "orders", "orders", ""},
		{"feature", "features/orders.md", "orders", ""},
		{"feature", "features/orders", "orders", ""},
		{"repo", "repos/shop-api.md", "shop-api", ""},
		{"repo", "features/orders.md", "", "target: features/orders.md is a feature page, but kind is repo"},
		{"feature", "docs/orders.md", "", "target: expected a page slug"},
	} {
		p, err := ParseProposal(fmt.Sprintf(front, c.kind, c.target))
		switch {
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s %s: error %v, want %q", c.kind, c.target, err, c.err)
		case c.err == "" && (err != nil || p.Target != c.want):
			t.Errorf("%s %s: target %q, %v", c.kind, c.target, p.Target, err)
		}
	}
}
