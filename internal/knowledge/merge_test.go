package knowledge

import "testing"

func proposal(heading, content string) Proposal {
	return Proposal{Kind: "feature", Target: "share", Repo: "notes-api", Branch: "main", Commit: "1a2b3c4d5e", Heading: heading, Content: content}
}

const marker = "<!-- verified notes-api@1a2b3c4 2026-03-01 -->"

func TestMergeAppendsToTheEndOfAnExistingSection(t *testing.T) {
	page := "---\ntitle: Share\n---\n\n# Share\n\n## Gotchas\n\nOld fact. <!-- verified notes-api@abcdef1 2025-01-01 -->\n\n\n## Decisions\n\nKept.\n"
	got := Merge(page, proposal("gotchas", "Links expire after 7 days."), "2026-03-01")
	want := "---\ntitle: Share\n---\n\n# Share\n\n## Gotchas\n\nOld fact. <!-- verified notes-api@abcdef1 2025-01-01 -->\n\n" +
		"Links expire after 7 days. " + marker + "\n\n## Decisions\n\nKept.\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMergeKeepsSubsectionsInsideTheSection(t *testing.T) {
	page := "# Share\n\n## Gotchas\n\nA.\n\n### Detail\n\nB.\n# Other\n"
	got := Merge(page, proposal("Gotchas", "C."), "2026-03-01")
	want := "# Share\n\n## Gotchas\n\nA.\n\n### Detail\n\nB.\n\nC. " + marker + "\n\n# Other\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMergeAddsAMissingSectionAtTheEnd(t *testing.T) {
	got := Merge("# Share\n\nIntro.\n\n\n", proposal("Notes", "- one\n- two"), "2026-03-01")
	want := "# Share\n\nIntro.\n\n## Notes\n\n- one\n- two " + marker + "\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMergeIgnoresHeadingsInFencesAndPutsTheMarkerAfterAFence(t *testing.T) {
	page := "# Share\n\n```\n## Gotchas\n```\n"
	got := Merge(page, proposal("Gotchas", "Run:\n\n```\nmake dev\n```"), "2026-03-01")
	want := "# Share\n\n```\n## Gotchas\n```\n\n## Gotchas\n\nRun:\n\n```\nmake dev\n```\n" + marker + "\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestNewPageKeepsTheTemplateFrontMatterAndHeadings(t *testing.T) {
	template := "---\ntitle: Feature name\nrepos: [repo-name]\ntags: []\n---\n\n# Feature name\n\n## Purpose\n\nWhat it is for.\n\n```\n## not a heading\n```\n## Gotchas\n- `<repo>:<path>:<line>`\n"
	got := NewPage(template, Proposal{Kind: "feature", Target: "share-links", Repo: "notes-api"})
	want := "---\ntitle: Share links\nrepos: [notes-api]\ntags: []\n---\n\n# Share links\n\n## Purpose\n\n## Gotchas\n"
	if got != want {
		t.Errorf("got\n%s", got)
	}
	if got := NewPage(template, Proposal{Kind: "repo", Target: "notes-api", Repo: "notes-api"}); Parse(got).Title != "notes-api" {
		t.Errorf("repo page title %q", Parse(got).Title)
	}
}

func TestPruneEmptySections(t *testing.T) {
	page := "---\ntitle: Orders\n---\n\n# Orders\n\n## Purpose\n\n## Entry points\n\n- `a:b:1`\n\n## Decisions\n\n### Why\n\n## Gotchas\n\n```md\n## Not a heading\n```\n\n## Cancel\n\nAtomic. <!-- verified a@1234567 2026-01-01 -->\n"
	want := "---\ntitle: Orders\n---\n\n# Orders\n\n## Entry points\n\n- `a:b:1`\n\n## Gotchas\n\n```md\n## Not a heading\n```\n\n## Cancel\n\nAtomic. <!-- verified a@1234567 2026-01-01 -->\n"
	if got := PruneEmptySections(page); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := PruneEmptySections(want); got != want {
		t.Errorf("pruning twice changed the page:\n%s", got)
	}
}
