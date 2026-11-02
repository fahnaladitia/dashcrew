// Package knowledgemcp serves the knowledge clone to agents as an MCP server over stdio: ranked search, page
// reads with their verified markers, and the state of a page's code pointers. `sync` registers it for every
// agent when the profile sets `knowledge`.
package knowledgemcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// ServerName is the name the server reports and the key sync registers it under.
const ServerName = "dashcrew-knowledge"

const (
	defaultLimit = 5
	maxLimit     = 20
)

const (
	searchDescription = "Search the team's reviewed product knowledge (the pages and specs of .ai/knowledge) for a feature, flow or question. " +
		"Ranked by BM25 over page title, headings and body; returns each page path, kind (spec, feature, repo or other, from its folder), " +
		"title, best matching snippet and score. Pass kind to keep one kind, such as spec for the PRD and TRD files under specs/. " +
		"Pages can be out of date: open a hit with knowledge_read and check its code pointers with knowledge_sources before relying on it."
	readDescription = "Read one knowledge page by the path knowledge_search returned, such as features/checkout.md. " +
		"Returns the page text and its verified markers (repo@sha date), which say when each fact was last checked against the code. " +
		"Before relying on a fact, run knowledge_sources and read the code it points to."
	sourcesDescription = "List the code pointers of one knowledge page as repo:path:line, each found (the path exists at HEAD of that repo), " +
		"missing (the path is gone, so the page is stale there) or unchecked (the repo is neither this project nor a sibling folder). " +
		"Read the found files at those lines before relying on the page; never rely on a missing or unchecked pointer. knowledge_sources checks paths, not lines."
	verifyHint = "Verify with knowledge_sources and the code before relying on a page."
)

// SearchInput are the arguments of knowledge_search.
type SearchInput struct {
	Query string `json:"query" jsonschema:"words to look for, such as a feature name or a question"`
	Repo  string `json:"repo,omitempty" jsonschema:"only pages whose front matter repos names this repo"`
	Kind  string `json:"kind,omitempty" jsonschema:"only pages of this kind: spec, feature, repo or other"`
	Limit int    `json:"limit,omitempty" jsonschema:"most pages to return, 1 to 20; default 5"`
}

// SearchOutput is the ranked pages, best first.
type SearchOutput struct {
	Results []SearchResult `json:"results"`
}

// SearchResult is one ranked page.
type SearchResult struct {
	Page    string  `json:"page"`
	Kind    string  `json:"kind"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// PageInput names one page, a path relative to the clone.
type PageInput struct {
	Page string `json:"page" jsonschema:"page path relative to the knowledge clone, such as features/checkout.md"`
}

// ReadOutput is one page and its verified markers.
type ReadOutput struct {
	Page     string           `json:"page"`
	Title    string           `json:"title"`
	Text     string           `json:"text"`
	Verified []VerifiedMarker `json:"verified"`
}

// VerifiedMarker is one `<!-- verified <repo>@<sha> <date> -->` marker; Line is its line in the page.
type VerifiedMarker struct {
	Repo string `json:"repo"`
	SHA  string `json:"sha"`
	Date string `json:"date"`
	Line int    `json:"line"`
}

// SourcesOutput is a page's code pointers.
type SourcesOutput struct {
	Page    string   `json:"page"`
	Sources []Source `json:"sources"`
}

// Source is one code pointer and its state: found, missing or unchecked.
type Source struct {
	Pointer  string `json:"pointer"`
	Status   string `json:"status"`
	PageLine int    `json:"page_line"`
}

// base is the clone and its search index, rebuilt when the clone's HEAD moves.
type base struct {
	clone, project string

	mu    sync.Mutex
	head  string
	index *knowledge.SearchIndex
}

// New returns the server for clone, a knowledge clone; project is where code pointers resolve (the project
// repo and its sibling folders). The index is built here and rebuilt by a call that finds the clone's HEAD moved.
func New(clone, project string) (*mcp.Server, error) {
	if info, err := os.Stat(clone); err != nil || !info.IsDir() {
		return nil, errors.New("No knowledge clone at " + clone + "; run dashcrew knowledge pull.")
	}
	b := &base{clone: clone, project: project}
	if _, err := b.searchIndex(); err != nil {
		return nil, err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: version.Current()}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "knowledge_search", Description: searchDescription}, b.search)
	mcp.AddTool(server, &mcp.Tool{Name: "knowledge_read", Description: readDescription}, b.read)
	mcp.AddTool(server, &mcp.Tool{Name: "knowledge_sources", Description: sourcesDescription}, b.sources)
	return server, nil
}

// Run serves clone over newline-delimited JSON-RPC on in and out, the process's stdin and stdout for an agent,
// until in ends or the client disconnects.
func Run(ctx context.Context, clone, project string, in io.Reader, out io.Writer) error {
	server, err := New(clone, project)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(in), Writer: nopCloser{out}})
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// searchIndex is the index of the clone's pages, rebuilt when the HEAD read from the clone's git files differs
// from the one it was built at. A clone whose HEAD cannot be read is indexed once.
func (b *base) searchIndex() (*knowledge.SearchIndex, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	head, _ := knowledge.HeadSHA(b.clone)
	if b.index != nil && head == b.head {
		return b.index, nil
	}
	pages, err := knowledge.SearchPages(b.clone)
	if err != nil {
		return nil, err
	}
	b.index, b.head = knowledge.NewSearchIndex(pages), head
	return b.index, nil
}

func (b *base) search(_ context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	out := SearchOutput{Results: []SearchResult{}}
	if strings.TrimSpace(in.Query) == "" {
		return nil, out, errors.New("query is empty; give words to look for")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind != "" && !slices.Contains(knowledge.Kinds, kind) {
		return nil, out, errors.New("kind " + kind + " is unknown; pass spec, feature, repo or other")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	index, err := b.searchIndex()
	if err != nil {
		return nil, out, err
	}
	var text strings.Builder
	for i, hit := range index.SearchKind(in.Query, strings.TrimSpace(in.Repo), kind, limit) {
		out.Results = append(out.Results, SearchResult{Page: hit.Path, Kind: hit.Kind, Title: hit.Title, Snippet: hit.Snippet, Score: hit.Score})
		fmt.Fprintf(&text, "%d. [%s] %s - %s (score %s)\n   %s\n", i+1, hit.Kind, hit.Path, hit.Title, strconv.FormatFloat(hit.Score, 'f', -1, 64), hit.Snippet)
	}
	if len(out.Results) == 0 {
		text.WriteString("No page matches; try other words, or read README.md in the knowledge clone for the index.\n")
	} else {
		text.WriteString(verifyHint + "\n")
	}
	return textResult(text.String()), out, nil
}

func (b *base) read(_ context.Context, _ *mcp.CallToolRequest, in PageInput) (*mcp.CallToolResult, ReadOutput, error) {
	out := ReadOutput{Verified: []VerifiedMarker{}}
	page, text, err := b.page(in.Page)
	if err != nil {
		return nil, out, err
	}
	parsed := knowledge.Parse(text)
	out.Page, out.Title, out.Text = page, parsed.Title, text
	var markers strings.Builder
	for _, v := range parsed.Verified {
		out.Verified = append(out.Verified, VerifiedMarker{Repo: v.Repo, SHA: v.SHA, Date: v.Date, Line: v.Line})
		fmt.Fprintf(&markers, "- line %d: %s@%s %s\n", v.Line, v.Repo, v.SHA, v.Date)
	}
	summary := "Verified markers: none, so treat every fact on this page as unverified.\n"
	if len(out.Verified) > 0 {
		summary = "Verified markers:\n" + markers.String()
	}
	return textResult(strings.TrimRight(text, "\n") + "\n\n" + summary + verifyHint + "\n"), out, nil
}

func (b *base) sources(_ context.Context, _ *mcp.CallToolRequest, in PageInput) (*mcp.CallToolResult, SourcesOutput, error) {
	out := SourcesOutput{Sources: []Source{}}
	page, text, err := b.page(in.Page)
	if err != nil {
		return nil, out, err
	}
	out.Page = page
	var lines strings.Builder
	for _, check := range knowledge.CheckPointers(b.project, knowledge.Parse(text).Pointers) {
		pointer := check.Pointer.String()
		out.Sources = append(out.Sources, Source{Pointer: pointer, Status: check.Status, PageLine: check.Pointer.Line})
		fmt.Fprintf(&lines, "%s  %s  (page line %d)\n", pointer, check.Status, check.Pointer.Line)
	}
	if len(out.Sources) == 0 {
		lines.WriteString("No code pointers on " + page + "; nothing on it can be checked against the code.\n")
	} else {
		lines.WriteString("Read the found files at those lines before relying on the page; a missing path means the page is stale there.\n")
	}
	return textResult(lines.String()), out, nil
}

// page reads a page of the clone; the path must stay inside it.
func (b *base) page(page string) (string, string, error) {
	rel, file, err := knowledge.PageFile(b.clone, page)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", "", err
	}
	return rel, strings.ReplaceAll(string(data), "\r\n", "\n"), nil
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
