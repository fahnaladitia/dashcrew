package spec

import (
	"reflect"
	"strings"
	"testing"
)

const goodPRD = `---
title: Notes sync
feature: notes-sync
release: "1.4.0"
status: draft
rev: 1
repos: [notes-api, notes-app]
owners: [product]
---

# Notes sync: PRD

## Acceptance criteria

- AC-1 Given two devices, when a note changes on one, then the other shows it within a minute.
- ~~AC-2~~ dropped: offline merge moves to a later release
- AC-3: Given no network, when the user edits, then the edit is kept.

## Not an ID

Text that mentions AC-9 in the middle of a sentence.

` + "```" + `
- AC-7 inside a fence is an example
` + "```" + `
`

func TestParseReadsFrontMatterAndIDs(t *testing.T) {
	doc := Parse(goodPRD)
	if len(doc.Problems) != 0 {
		t.Fatalf("problems: %v", doc.Problems)
	}
	if doc.Title != "Notes sync" || doc.Feature != "notes-sync" || doc.Release != "1.4.0" || doc.Status != Draft || doc.Rev != 1 {
		t.Errorf("front matter: %+v", doc)
	}
	if !reflect.DeepEqual(doc.Repos, []string{"notes-api", "notes-app"}) || !reflect.DeepEqual(doc.Owners, []string{"product"}) {
		t.Errorf("lists: %v %v", doc.Repos, doc.Owners)
	}
	var got []string
	for _, id := range doc.IDs {
		got = append(got, id.String()+"|"+id.Text+"|"+strings.TrimSpace(id.Reason))
	}
	want := []string{
		"AC-1|Given two devices, when a note changes on one, then the other shows it within a minute.|",
		"AC-2||offline merge moves to a later release",
		"AC-3|Given no network, when the user edits, then the edit is kept.|",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ids:\n%v\nwant\n%v", got, want)
	}
	if !doc.IDs[1].Dropped || doc.IDs[0].Dropped || doc.IDs[1].Line != 16 {
		t.Errorf("dropped flags or line: %+v", doc.IDs[1])
	}
}

func TestIDsAtTheStartOfHeadingsAndNumberedItems(t *testing.T) {
	doc := Parse("---\n---\n## D-1 Store notes as rows\n1. C-2 `GET /notes`\n* AC-12a is not an ID\n- AC-01 is not an ID\n- ~~D-3 old text~~ dropped: superseded by D-1\n")
	var got []string
	for _, id := range doc.IDs {
		got = append(got, id.String()+"|"+id.Text)
	}
	if !reflect.DeepEqual(got, []string{"D-1|Store notes as rows", "C-2|`GET /notes`", "D-3|old text"}) {
		t.Errorf("%v", got)
	}
}

func TestParseProblems(t *testing.T) {
	cases := map[string]string{
		"no front matter":              "# x\n",
		"front matter is not closed":   "---\ntitle: x\n",
		"front matter has no feature":  "---\ntitle: x\nrelease: r\nstatus: draft\nrev: 1\nrepos: []\nowners: []\n---\n",
		"status done is not one of":    "---\ntitle: x\nfeature: f\nrelease: r\nstatus: done\nrev: 1\nrepos: []\nowners: []\n---\n",
		"rev must be a whole number":   "---\ntitle: x\nfeature: f\nrelease: r\nstatus: draft\nrev: 0\nrepos: []\nowners: []\n---\n",
		"owners hold an email address": "---\ntitle: x\nfeature: f\nrelease: r\nstatus: draft\nrev: 1\nrepos: []\nowners: [ana@example.com]\n---\n",
		"release 1.4 is a number":      "---\ntitle: x\nfeature: f\nrelease: 1.4\nstatus: draft\nrev: 1\nrepos: []\nowners: []\n---\n",
		"struck through without a reason; write `~~AC-1~~ dropped: <why>`": "---\n---\n- ~~AC-1~~ gone\n",
	}
	for want, text := range cases {
		doc := Parse(text)
		found := false
		for _, p := range doc.Problems {
			found = found || strings.Contains(p.Message, want)
		}
		if !found {
			t.Errorf("%q: %v", want, doc.Problems)
		}
	}
}

func TestOpenQuestions(t *testing.T) {
	doc := Parse("---\n---\n## Open questions\n\n- How often do we sync? owner: backend\n- Who pays for storage?\n" +
		"  It decides the quota. owner: product\n- ~~Do we need export?~~ resolved: no\n- none\n- Retention? owner: ana@example.com\n\n## Risks\n\n- Not a question\n")
	var got []string
	for _, q := range doc.Questions {
		got = append(got, q.Owner)
	}
	if !reflect.DeepEqual(got, []string{"backend", "product", "ana@example.com"}) {
		t.Errorf("%v", got)
	}
	if doc.Questions[1].Line != 6 {
		t.Errorf("line %d", doc.Questions[1].Line)
	}
}

func lintMessages(problems []Problem) []string {
	var out []string
	for _, p := range problems {
		out = append(out, p.String())
	}
	return out
}

func TestLintGoodSpecHasNoProblems(t *testing.T) {
	if got := Lint(Parse(goodPRD), nil, true); len(got) != 0 {
		t.Errorf("%v", lintMessages(got))
	}
}

func TestLintDuplicateAndNonSequentialIDs(t *testing.T) {
	text := strings.Replace(goodPRD, "- AC-3:", "- AC-1 again\n- AC-5:", 1)
	got := lintMessages(Lint(Parse(text), nil, true))
	want := []string{
		"line 17: AC-1 is defined twice (first on line 15)",
		"line 18: AC IDs skip AC-3, AC-4: a new spec numbers them from AC-1 without gaps",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%v", got)
	}
	if got := Lint(Parse(text), nil, false); len(got) != 1 {
		t.Errorf("a spec that is not new keeps its gaps: %v", lintMessages(got))
	}
}

func TestLintApprovedNeedsOwnedQuestions(t *testing.T) {
	text := "---\ntitle: x\nfeature: f\nrelease: r\nstatus: approved\nrev: 2\nrepos: []\nowners: []\n---\n## Open questions\n- Sync interval?\n- Quota? owner: product\n"
	got := lintMessages(Lint(Parse(text), nil, false))
	if !reflect.DeepEqual(got, []string{"line 11: open question without an owner on an approved spec; end it with owner: <role>"}) {
		t.Errorf("%v", got)
	}
	if got := Lint(Parse(strings.Replace(text, "approved", "review", 1)), nil, false); len(got) != 0 {
		t.Errorf("review: %v", lintMessages(got))
	}
}

func TestLintAgainstABase(t *testing.T) {
	base := Parse(strings.Replace(goodPRD, "status: draft", "status: approved", 1))
	cases := map[string]struct {
		text string
		want []string
	}{
		"unchanged": {strings.Replace(goodPRD, "status: draft", "status: approved", 1), nil},
		"vanished": {strings.Replace(strings.Replace(goodPRD, "rev: 1", "rev: 2", 1), "- AC-3: Given no network, when the user edits, then the edit is kept.\n", "", 1),
			[]string{"AC-3 is gone (it was on line 17 at main); keep it struck through: `~~AC-3~~ dropped: <why>`"}},
		"renumbered": {strings.Replace(strings.Replace(goodPRD, "rev: 1", "rev: 2", 1), "- AC-3:", "- AC-4:", 1),
			[]string{"AC-3 is gone (it was on line 17 at main); keep it struck through: `~~AC-3~~ dropped: <why>`",
				"line 17: AC-4 holds the text of AC-3 at main; IDs are never renumbered"}},
		"reused": {strings.Replace(strings.Replace(goodPRD, "rev: 1", "rev: 2", 1), "- ~~AC-2~~ dropped: offline merge moves to a later release", "- AC-2 Given a conflict, then both versions stay.", 1),
			[]string{"line 16: AC-2 was dropped at main; a dropped ID is never reused, number a new one"}},
		"approved edited without rev": {strings.Replace(strings.Replace(goodPRD, "status: draft", "status: approved", 1), "within a minute", "within a second", 1),
			[]string{"line 6: the file was approved at main and changed without a rev increase; a member runs dashcrew spec revise first"}},
		"revised":    {strings.Replace(strings.Replace(goodPRD, "rev: 1", "rev: 2", 1), "within a minute", "within a second", 1), nil},
		"superseded": {strings.Replace(goodPRD, "status: draft", "status: superseded", 1), nil},
	}
	for name, c := range cases {
		got := lintMessages(Lint(Parse(c.text), &Base{Doc: base, Text: strings.Replace(goodPRD, "status: draft", "status: approved", 1), Ref: "main"}, false))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n%q\nwant\n%q", name, got, c.want)
		}
	}
}

func TestReadStatus(t *testing.T) {
	for text, want := range map[string]string{
		"---\ntitle: x\nstatus: approved\n---\n":                    "approved",
		"---\r\nstatus: \"review\"  # comment\r\n---\r\n":           "review",
		"---\nstatus: 'draft'\n---\n":                               "draft",
		"# no front matter\nstatus: draft\n":                        "",
		"---\ntitle: x\n---\nstatus: draft\n":                       "",
		"---\n" + strings.Repeat("x: y\n", 600) + "status: draft\n": "",
	} {
		if got := StatusOf([]byte(text)); got != want {
			t.Errorf("%q: %q, want %q", text[:min(len(text), 40)], got, want)
		}
	}
}
