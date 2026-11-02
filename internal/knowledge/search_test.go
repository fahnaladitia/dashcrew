package knowledge

import (
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
