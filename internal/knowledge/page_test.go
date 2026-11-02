package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, name string) Page {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "pages", name))
	if err != nil {
		t.Fatal(err)
	}
	return Parse(string(data))
}

func problemsText(page Page) string {
	var lines []string
	for _, problem := range page.Problems {
		lines = append(lines, problem.String())
	}
	return strings.Join(lines, "\n")
}

func TestParseReadsFrontMatterHeadingsMarkersAndPointers(t *testing.T) {
	page := parseFixture(t, "full.md")
	if !page.FrontMatter || page.Title != "Note sharing" {
		t.Errorf("front matter %v, title %q", page.FrontMatter, page.Title)
	}
	if !reflect.DeepEqual(page.Repos, []string{"notes-api", "notes-app"}) || !reflect.DeepEqual(page.Tags, []string{"sharing", "permissions"}) {
		t.Errorf("repos %v, tags %v", page.Repos, page.Tags)
	}
	wantHeadings := []Heading{{1, "Note sharing", 7}, {2, "Purpose", 9}, {2, "Entry points", 13}, {3, "Gotchas", 24}}
	if !reflect.DeepEqual(page.Headings, wantHeadings) {
		t.Errorf("headings %+v", page.Headings)
	}
	wantVerified := []Verified{
		{Repo: "notes-api", SHA: "3f2a9c1", Date: "2026-10-30", Line: 11},
		{Repo: "notes-app", SHA: "a1b2c3d4e5f6", Date: "2026-10-29", Line: 16},
		{Repo: "notes-api", SHA: "3f2a9c1", Date: "2026-10-30", Line: 26},
	}
	if !reflect.DeepEqual(page.Verified, wantVerified) {
		t.Errorf("verified %+v", page.Verified)
	}
	wantPointers := []Pointer{
		{Repo: "notes-api", Path: "internal/share/handler.go", CodeLine: 42, Line: 15},
		{Repo: "notes-app", Path: "lib/share/share_sheet.dart", CodeLine: 17, Line: 16},
		{Repo: "notes-api", Path: "internal/share/expiry.go", CodeLine: 9, Line: 26},
	}
	if !reflect.DeepEqual(page.Pointers, wantPointers) {
		t.Errorf("pointers %+v", page.Pointers)
	}
	if len(page.Problems) != 0 {
		t.Errorf("problems:\n%s", problemsText(page))
	}
	if !strings.HasPrefix(page.Body, "\n# Note sharing\n") {
		t.Errorf("body starts %q", page.Body[:20])
	}
	if got := page.Pointers[0].String(); got != "notes-api:internal/share/handler.go:42" {
		t.Errorf("pointer string %q", got)
	}
}

func TestPageWithoutFrontMatterTakesItsTitleFromTheFirstHeading(t *testing.T) {
	page := parseFixture(t, "no-front-matter.md")
	if page.FrontMatter || page.Title != "Offline sync" || page.Repos != nil || page.Tags != nil {
		t.Errorf("%+v", page)
	}
	if len(page.Verified) != 1 || page.Verified[0].SHA != "0badc0de" || len(page.Pointers) != 1 || page.Pointers[0].CodeLine != 120 {
		t.Errorf("verified %+v, pointers %+v", page.Verified, page.Pointers)
	}
	if got := problemsText(page); got != "line 1: no front matter; start the page with --- title, repos, tags ---" {
		t.Errorf("problems:\n%s", got)
	}
	if page.Body != readFixture(t, "no-front-matter.md") {
		t.Error("body is not the whole page")
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "pages", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMalformedMarkersArePageProblemsAndOnlyTheValidOneCounts(t *testing.T) {
	page := parseFixture(t, "malformed-markers.md")
	if len(page.Verified) != 1 || page.Verified[0].Line != 15 {
		t.Errorf("verified %+v", page.Verified)
	}
	want := strings.Join([]string{
		"line 9: malformed verified marker; want <!-- verified <repo>@<sha> <yyyy-mm-dd> -->",
		"line 10: malformed verified marker; want <!-- verified <repo>@<sha> <yyyy-mm-dd> -->",
		"line 11: malformed verified marker; want <!-- verified <repo>@<sha> <yyyy-mm-dd> -->",
		"line 12: malformed verified marker; want <!-- verified <repo>@<sha> <yyyy-mm-dd> -->",
		"line 13: verified marker date 2026-13-01 is not a calendar date",
		"line 14: verified marker is not closed with -->",
	}, "\n")
	if got := problemsText(page); got != want {
		t.Errorf("problems:\n%s\nwant:\n%s", got, want)
	}
	if len(page.Pointers) != 0 {
		t.Errorf("pointers without a line number: %+v", page.Pointers)
	}
}

func TestFrontMatterOfTheWrongTypeIsAProblemNotAFailure(t *testing.T) {
	page := parseFixture(t, "bad-front-matter.md")
	want := strings.Join([]string{
		"line 1: front matter title: expected a string",
		"line 1: front matter repos: expected a list of names",
		"line 1: front matter tags: expected a list of names",
	}, "\n")
	if got := problemsText(page); got != want {
		t.Errorf("problems:\n%s", got)
	}
	if !page.FrontMatter || page.Title != "Search" || page.Repos != nil || page.Tags != nil {
		t.Errorf("%+v", page)
	}
	if len(page.Headings) != 1 || page.Headings[0].Line != 8 {
		t.Errorf("headings %+v", page.Headings)
	}
}

func TestUnclosedFrontMatterIsReadAsBody(t *testing.T) {
	page := parseFixture(t, "unclosed-front-matter.md")
	if page.FrontMatter || page.Title != "Export" {
		t.Errorf("%+v", page)
	}
	if got := problemsText(page); got != "line 1: front matter is not closed with ---" {
		t.Errorf("problems:\n%s", got)
	}
}

func TestInvalidFrontMatterYamlAndCRLF(t *testing.T) {
	page := Parse("---\r\ntitle: [unclosed\r\n---\r\n# Title\r\n")
	if !page.FrontMatter || page.Title != "Title" || len(page.Problems) != 1 ||
		!strings.HasPrefix(page.Problems[0].String(), "line 1: front matter is not valid YAML") {
		t.Errorf("%+v", page)
	}
	page = Parse("---\r\ntitle: Notes\r\n---\r\n\r\n## Part\r\n")
	if page.Title != "Notes" || len(page.Headings) != 1 || page.Headings[0] != (Heading{2, "Part", 5}) || len(page.Problems) != 0 {
		t.Errorf("%+v", page)
	}
	page = Parse("---\ntitle: Notes\n---\n")
	if page.Title != "Notes" || page.Body != "" || len(page.Problems) != 0 {
		t.Errorf("%+v", page)
	}
	page = Parse("---\n- a list\n---\n# T\n")
	if got := problemsText(page); got != "line 1: front matter is not a map" {
		t.Errorf("problems:\n%s", got)
	}
	page = Parse("---\nrepos: [a]\n---\n")
	if got := problemsText(page); got != "line 1: front matter has no title" {
		t.Errorf("problems:\n%s", got)
	}
}

func TestHeadingsNeedASpaceAndTildeFencesHideContent(t *testing.T) {
	page := Parse("---\ntitle: T\n---\n#hashtag\n####### seven\n~~~\n## hidden\n```\n## still hidden\n~~~\n  ## Indented ##  \n")
	if !reflect.DeepEqual(page.Headings, []Heading{{2, "Indented", 11}}) {
		t.Errorf("headings %+v", page.Headings)
	}
}
