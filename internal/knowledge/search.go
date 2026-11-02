package knowledge

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Search fields, in the order of FieldWeights.
const (
	fieldTitle = iota
	fieldHeadings
	fieldBody
	fieldCount
)

// FieldWeights multiply a term's frequency per field before BM25 saturates it: a word in the title counts as
// three in the body, a word in a heading as two.
var FieldWeights = [fieldCount]float64{fieldTitle: 3, fieldHeadings: 2, fieldBody: 1}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
	// snippetMax is the longest snippet in runes before it is cut at a word.
	snippetMax = 240
)

// stopWords carry no meaning in a question and are never indexed.
var stopWords = map[string]bool{}

func init() {
	for _, word := range strings.Fields(`a an and are as at be been but by can could did do does for from had has
		have how i if in into is it its me my of on or our so than that the their them then there these they this
		those to was we were what when where which who why will with would you your`) {
		stopWords[word] = true
	}
}

// SearchIndex ranks pages with BM25F over their title, headings and body. It is built once from the pages and
// never reads the disk.
type SearchIndex struct {
	docs      []searchDoc
	documents map[string]int
	average   [fieldCount]float64
}

type searchDoc struct {
	path, title string
	repos       []string
	terms       [fieldCount]map[string]int
	length      [fieldCount]int
	paragraphs  []string
}

// SearchHit is one ranked page; Snippet is its paragraph that matches the query best, markers removed.
type SearchHit struct {
	Path, Title, Snippet string
	Score                float64
}

// NewSearchIndex indexes pages as Pages returns them.
func NewSearchIndex(pages []IndexedPage) *SearchIndex {
	index := &SearchIndex{documents: map[string]int{}}
	var total [fieldCount]int
	for _, page := range pages {
		doc := searchDoc{path: page.Path, title: page.Title, repos: page.Page.Repos}
		var headings []string
		for _, h := range page.Page.Headings {
			headings = append(headings, h.Text)
		}
		body, paragraphs := bodyText(page.Page.Body)
		doc.paragraphs = paragraphs
		seen := map[string]bool{}
		for field, text := range [fieldCount]string{page.Title, strings.Join(headings, "\n"), body} {
			doc.terms[field] = map[string]int{}
			for _, term := range tokens(text) {
				doc.terms[field][term]++
				doc.length[field]++
				if !seen[term] {
					seen[term] = true
					index.documents[term]++
				}
			}
			total[field] += doc.length[field]
		}
		index.docs = append(index.docs, doc)
	}
	for field := range total {
		if len(index.docs) > 0 {
			index.average[field] = float64(total[field]) / float64(len(index.docs))
		}
	}
	return index
}

// Search returns at most limit pages that share a word with query, best first, ties by path. A non-empty repo
// keeps only pages whose front matter `repos` names it.
func (x *SearchIndex) Search(query, repo string, limit int) []SearchHit {
	terms := unique(tokens(query))
	if len(terms) == 0 || limit <= 0 {
		return nil
	}
	var hits []SearchHit
	for _, doc := range x.docs {
		if repo != "" && !contains(doc.repos, repo) {
			continue
		}
		score := 0.0
		for _, term := range terms {
			weighted := 0.0
			for field := range doc.terms {
				tf := doc.terms[field][term]
				if tf == 0 {
					continue
				}
				norm := 1.0
				if x.average[field] > 0 {
					norm = 1 - bm25B + bm25B*float64(doc.length[field])/x.average[field]
				}
				weighted += FieldWeights[field] * float64(tf) / norm
			}
			if weighted > 0 {
				score += x.idf(term) * weighted / (bm25K1 + weighted)
			}
		}
		if score > 0 {
			hits = append(hits, SearchHit{Path: doc.path, Title: doc.title, Snippet: x.snippet(doc, terms), Score: math.Round(score*1000) / 1000})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// idf is the BM25 inverse document frequency, always positive.
func (x *SearchIndex) idf(term string) float64 {
	n, df := float64(len(x.docs)), float64(x.documents[term])
	return math.Log(1 + (n-df+0.5)/(df+0.5))
}

// snippet is the paragraph holding the most query weight (the sum of the idf of the query words it contains),
// the first paragraph when none does; cut at a word past snippetMax runes.
func (x *SearchIndex) snippet(doc searchDoc, terms []string) string {
	best, bestScore := "", 0.0
	for i, paragraph := range doc.paragraphs {
		words := map[string]bool{}
		for _, term := range tokens(paragraph) {
			words[term] = true
		}
		score := 0.0
		for _, term := range terms {
			if words[term] {
				score += x.idf(term)
			}
		}
		if i == 0 || score > bestScore {
			best, bestScore = paragraph, score
		}
	}
	runes := []rune(best)
	if len(runes) <= snippetMax {
		return best
	}
	cut := string(runes[:snippetMax])
	if space := strings.LastIndex(cut, " "); space > 0 {
		cut = cut[:space]
	}
	return cut + " ..."
}

// bodyText is the body without headings, HTML comments and fence lines, and its paragraphs with spaces collapsed.
func bodyText(body string) (string, []string) {
	var lines, paragraphs, current []string
	flush := func() {
		if len(current) > 0 {
			paragraphs = append(paragraphs, strings.Join(current, " "))
			current = nil
		}
	}
	for _, line := range strings.Split(comment.ReplaceAllString(body, ""), "\n") {
		text := strings.TrimSpace(collapseSpace.ReplaceAllString(line, " "))
		switch {
		case heading.MatchString(line) || fence.MatchString(line) || text == "":
			flush()
		default:
			lines = append(lines, text)
			current = append(current, text)
		}
	}
	flush()
	return strings.Join(lines, "\n"), paragraphs
}

// tokens are the lower-case words of text split at anything but a letter or digit, stop words and one-letter
// words dropped, each reduced by stem.
func tokens(text string) []string {
	var out []string
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(word)) < 2 || stopWords[word] {
			continue
		}
		out = append(out, stem(word))
	}
	return out
}

// stem drops the common English inflections, so "refunds" finds "refund" and "expires" finds "expire". It is
// deliberately crude: the query and the pages go through the same rules. A final `e` goes too, so "expire",
// "expires" and "expired" meet.
func stem(word string) string {
	switch n := len(word); {
	case n > 4 && strings.HasSuffix(word, "ies"):
		return word[:n-3] + "y"
	case n > 5 && strings.HasSuffix(word, "ing"):
		word = word[:n-3]
	case n > 4 && strings.HasSuffix(word, "ed"):
		word = word[:n-2]
	case n > 3 && strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us"):
		word = word[:n-1]
	}
	if len(word) > 4 && strings.HasSuffix(word, "e") {
		word = word[:len(word)-1]
	}
	return word
}

func unique(words []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, word := range words {
		if !seen[word] {
			seen[word] = true
			out = append(out, word)
		}
	}
	return out
}
