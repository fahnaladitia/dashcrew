package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func searchFixture(t *testing.T) *SearchIndex {
	t.Helper()
	pages, err := Pages("testdata/search")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 40 {
		t.Fatalf("fixture has %d pages, want 40", len(pages))
	}
	return NewSearchIndex(pages)
}

// TestSearchQualityOverTheFixtureRepo asks 20 questions; the page that answers each must rank in the top 3 for
// at least 17 of them.
func TestSearchQualityOverTheFixtureRepo(t *testing.T) {
	index := searchFixture(t)
	questions := []struct{ question, page string }{
		{"How long is a password reset link valid?", "features/password-reset.md"},
		{"What happens after several failed sign in attempts?", "features/sign-in.md"},
		{"How do recovery codes work for two-factor authentication?", "features/two-factor.md"},
		{"How often does the note editor autosave?", "features/note-editor.md"},
		{"Can a shared note link expire?", "features/note-sharing.md"},
		{"How are sync conflicts resolved after editing offline?", "features/offline-sync.md"},
		{"How long can a deleted note be restored from the trash?", "features/trash.md"},
		{"When is the next occurrence of a recurring todo created?", "features/recurring-tasks.md"},
		{"Which roles can members of a shared list have?", "features/collaboration.md"},
		{"How long is a cart kept before it expires?", "features/cart.md"},
		{"Which steps does checkout go through?", "features/checkout.md"},
		{"How does 3-D Secure work for card payments?", "features/payments.md"},
		{"How long does a refund take to reach the card?", "features/refunds.md"},
		{"Can two discount codes be combined on one order?", "features/discount-codes.md"},
		{"When is shipping free?", "features/shipping-rates.md"},
		{"When is reserved stock released during checkout?", "features/inventory.md"},
		{"How do I download the invoice of an order?", "features/order-history.md"},
		{"How long is the free trial of the premium plan?", "features/subscription-plans.md"},
		{"What is the grace period before an account is deleted?", "features/account-deletion.md"},
		{"Which job retries failed work with backoff?", "repos/jobs-worker.md"},
	}
	inTop3 := 0
	for _, q := range questions {
		hits := index.Search(q.question, "", 3)
		found := false
		var got []string
		for _, hit := range hits {
			got = append(got, hit.Path)
			found = found || hit.Path == q.page
		}
		if found {
			inTop3++
		} else {
			t.Logf("miss: %q -> %v, want %s", q.question, got, q.page)
		}
	}
	t.Logf("%d/%d in the top 3", inTop3, len(questions))
	if inTop3 < 17 {
		t.Errorf("%d/%d questions have their page in the top 3, want at least 17", inTop3, len(questions))
	}
}

func TestSearchWeighsTheTitleOverTheBody(t *testing.T) {
	index := NewSearchIndex([]IndexedPage{
		testPage(t, "features/body.md", "---\ntitle: Labels\n---\n\nA refund is mentioned once in this body text.\n"),
		testPage(t, "features/title.md", "---\ntitle: Refund\n---\n\nThe money goes back to the card in this body text.\n"),
	})
	hits := index.Search("refund", "", 5)
	if len(hits) != 2 || hits[0].Path != "features/title.md" || hits[0].Score <= hits[1].Score {
		t.Errorf("hits %+v", hits)
	}
}

func TestSearchFiltersByRepoAndLimit(t *testing.T) {
	index := searchFixture(t)
	pages, err := Pages("testdata/search")
	if err != nil {
		t.Fatal(err)
	}
	inRepo := map[string]bool{}
	for _, p := range pages {
		inRepo[p.Path] = contains(p.Page.Repos, "shop-api")
	}
	hits := index.Search("checkout payment order products", "shop-api", 20)
	if len(hits) < 3 {
		t.Fatalf("hits %+v", hits)
	}
	for _, hit := range hits {
		if !inRepo[hit.Path] {
			t.Errorf("%s is not in shop-api", hit.Path)
		}
	}
	if got := index.Search("checkout payment order", "", 2); len(got) != 2 {
		t.Errorf("limit 2 gave %d", len(got))
	}
	if got := index.Search("checkout", "no-such-repo", 5); len(got) != 0 {
		t.Errorf("unknown repo gave %+v", got)
	}
}

func TestSearchSnippetIsTheBestParagraphWithoutMarkers(t *testing.T) {
	hits := searchFixture(t).Search("reset token expires", "", 1)
	if len(hits) != 1 || hits[0].Path != "features/password-reset.md" {
		t.Fatalf("hits %+v", hits)
	}
	if want := "The reset link carries a single-use token that expires after 30 minutes."; !strings.HasPrefix(hits[0].Snippet, want) {
		t.Errorf("snippet %q", hits[0].Snippet)
	}
	if strings.Contains(hits[0].Snippet, "<!--") || hits[0].Title != "Password reset" {
		t.Errorf("hit %+v", hits[0])
	}
}

func TestSearchWithoutMatchingWordsIsEmpty(t *testing.T) {
	index := searchFixture(t)
	for _, query := range []string{"", "  ", "the and of", "zebra"} {
		if hits := index.Search(query, "", 5); len(hits) != 0 {
			t.Errorf("%q gave %+v", query, hits)
		}
	}
}

func TestTokensStemPluralsAndDropStopWords(t *testing.T) {
	got := strings.Join(tokens("How do the Refunds, codes and replies work?"), " ")
	if got != "refund code reply work" {
		t.Errorf("tokens %q", got)
	}
}

func testPage(t *testing.T, path, text string) IndexedPage {
	t.Helper()
	parsed := Parse(text)
	return IndexedPage{Path: path, Page: parsed, Title: parsed.Title}
}

func TestKindOfIsTheFirstFolder(t *testing.T) {
	for path, want := range map[string]string{
		"specs/1.4.0/notes-sync/prd.md": KindSpec, "features/share.md": KindFeature, "repos/notes-api.md": KindRepo,
		"README.md": KindOther, "inbox/x.md": KindOther, "featuresx/y.md": KindOther,
	} {
		if got := KindOf(path); got != want {
			t.Errorf("%s: %s", path, got)
		}
	}
}

func TestSearchPagesAddSpecFilesAndSearchFiltersByKind(t *testing.T) {
	clone := t.TempDir()
	for path, text := range map[string]string{
		"features/sync.md":                 "---\ntitle: Notes sync\nrepos: [notes-app]\n---\n\nNotes sync across devices by cursor.\n",
		"repos/notes-app.md":               "---\ntitle: notes-app\nrepos: [notes-app]\n---\n\nThe app syncs notes on start.\n",
		"specs/1.4.0/notes-sync/prd.md":    "---\ntitle: Notes sync\nfeature: notes-sync\nstatus: approved\nrev: 2\nrepos: [notes-app]\n---\n\n- AC-1 Given two devices, when a note changes, then the other syncs it.\n",
		"specs/1.4.0/notes-sync/trd.md":    "---\ntitle: Notes sync\nstatus: approved\nrev: 2\nrepos: [notes-api]\n---\n\n- D-1 Sync by cursor.\n",
		"specs/1.4.0/notes-sync/notes.txt": "sync\n",
		"inbox/2026-01-01-sync.md":         "sync proposal\n",
	} {
		file := filepath.Join(clone, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := SearchPages(clone)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, page := range pages {
		paths = append(paths, page.Path)
	}
	if want := []string{"features/sync.md", "repos/notes-app.md", "specs/1.4.0/notes-sync/prd.md", "specs/1.4.0/notes-sync/trd.md"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("pages %v", paths)
	}
	index := NewSearchIndex(pages)
	kinds := map[string]string{}
	for _, hit := range index.Search("notes sync", "", 10) {
		kinds[hit.Path] = hit.Kind
	}
	if want := map[string]string{"features/sync.md": KindFeature, "repos/notes-app.md": KindRepo,
		"specs/1.4.0/notes-sync/prd.md": KindSpec, "specs/1.4.0/notes-sync/trd.md": KindSpec}; !reflect.DeepEqual(kinds, want) {
		t.Errorf("kinds %v", kinds)
	}
	for kind, want := range map[string][]string{
		KindSpec:    {"specs/1.4.0/notes-sync/prd.md", "specs/1.4.0/notes-sync/trd.md"},
		KindFeature: {"features/sync.md"},
		KindOther:   nil,
	} {
		var got []string
		for _, hit := range index.SearchKind("notes sync", "", kind, 10) {
			got = append(got, hit.Path)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v", kind, got)
		}
	}
	if got := index.SearchKind("sync", "notes-api", KindSpec, 10); len(got) != 1 || got[0].Path != "specs/1.4.0/notes-sync/trd.md" {
		t.Errorf("repo and kind: %+v", got)
	}
}
