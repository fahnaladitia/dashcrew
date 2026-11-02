package knowledgemcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	shaOne = "1111111111111111111111111111111111111111"
	shaTwo = "2222222222222222222222222222222222222222"
)

const sharePage = "---\ntitle: Share links\nrepos: [notes-app]\ntags: [sharing]\n---\n\n# Share links\n\n" +
	"A note is shared by link with read-only or edit permission. <!-- verified notes-app@abc1234 2026-09-01 -->\n\n" +
	"## Expiry\n\nA share link expires after one week. See `notes-app:src/share.go:3`, `notes-app:src/gone.go:1` " +
	"and `notes-api:src/link.go:9`. <!-- verified notes-app@abc1234 2026-09-02 -->\n"

func write(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is a knowledge clone with a detached HEAD and two pages, and a project folder named notes-app (not a
// git repo, so pointers are looked up on disk) holding src/share.go.
func fixture(t *testing.T) (clone, project string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clone, project = filepath.Join(root, "kb"), filepath.Join(root, "notes-app")
	write(t, clone, ".git/HEAD", shaOne+"\n")
	write(t, clone, "features/share.md", sharePage)
	write(t, clone, "features/tags.md", "---\ntitle: Tags\nrepos: [notes-app]\n---\n\nNotes carry colored tags.\n")
	write(t, clone, "inbox/.gitkeep", "")
	write(t, root, "secret.md", "secret\n")
	write(t, project, "src/share.go", "package share\n")
	return clone, project
}

func connect(t *testing.T, clone, project string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server, err := New(clone, project)
	if err != nil {
		t.Fatal(err)
	}
	serverSide, clientSide := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		session.Close()
		serverSession.Wait()
	})
	return session
}

// call runs a tool and decodes its structured result into out; it returns the text content and IsError.
func call(t *testing.T, session *mcp.ClientSession, tool string, args map[string]any, out any) (string, bool) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if c, ok := content.(*mcp.TextContent); ok {
			text.WriteString(c.Text)
		}
	}
	if out != nil && !result.IsError {
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("%s: %v in %s", tool, err, data)
		}
	}
	return text.String(), result.IsError
}

func TestTheServerOffersThreeToolsThatAskToVerifyPointers(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	var names []string
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
		if !strings.Contains(tool.Description, "knowledge_sources") {
			t.Errorf("%s description does not point to knowledge_sources: %q", tool.Name, tool.Description)
		}
	}
	if want := []string{"knowledge_read", "knowledge_search", "knowledge_sources"}; !reflect.DeepEqual(names, want) {
		t.Errorf("tools %v", names)
	}
}

func TestKnowledgeSearchRanksPagesWithASnippet(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	var out SearchOutput
	text, isError := call(t, session, "knowledge_search", map[string]any{"query": "when does a share link expire"}, &out)
	if isError || len(out.Results) == 0 {
		t.Fatalf("%v %s", isError, text)
	}
	first := out.Results[0]
	if first.Page != "features/share.md" || first.Title != "Share links" || first.Score <= 0 ||
		first.Snippet != "A share link expires after one week. See `notes-app:src/share.go:3`, `notes-app:src/gone.go:1` and `notes-api:src/link.go:9`." {
		t.Errorf("first %+v", first)
	}
	if !strings.Contains(text, "features/share.md") || !strings.Contains(text, "knowledge_sources") {
		t.Errorf("text %q", text)
	}

	call(t, session, "knowledge_search", map[string]any{"query": "notes", "repo": "notes-api"}, &out)
	if len(out.Results) != 0 {
		t.Errorf("repo filter %+v", out.Results)
	}
	call(t, session, "knowledge_search", map[string]any{"query": "notes link tags", "limit": 1}, &out)
	if len(out.Results) != 1 {
		t.Errorf("limit %+v", out.Results)
	}
	if text, isError := call(t, session, "knowledge_search", map[string]any{"query": "zebra"}, &out); isError || len(out.Results) != 0 || !strings.Contains(text, "No page matches") {
		t.Errorf("no match: %v %q %+v", isError, text, out.Results)
	}
	if _, isError := call(t, session, "knowledge_search", map[string]any{"query": " "}, nil); !isError {
		t.Error("empty query accepted")
	}
}

func TestKnowledgeReadReturnsThePageAndItsMarkers(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	var out ReadOutput
	text, isError := call(t, session, "knowledge_read", map[string]any{"page": "features/share"}, &out)
	if isError {
		t.Fatal(text)
	}
	want := []VerifiedMarker{{Repo: "notes-app", SHA: "abc1234", Date: "2026-09-01", Line: 9}, {Repo: "notes-app", SHA: "abc1234", Date: "2026-09-02", Line: 13}}
	if out.Page != "features/share.md" || out.Title != "Share links" || out.Text != sharePage || !reflect.DeepEqual(out.Verified, want) {
		t.Errorf("read %+v", out)
	}
	if !strings.HasPrefix(text, sharePage) || !strings.Contains(text, "line 13: notes-app@abc1234 2026-09-02") {
		t.Errorf("text %q", text)
	}
	call(t, session, "knowledge_read", map[string]any{"page": "features/tags.md"}, &out)
	if len(out.Verified) != 0 || out.Verified == nil {
		t.Errorf("unverified page %+v", out)
	}
}

func TestKnowledgeSourcesMarksEachPointer(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	var out SourcesOutput
	text, isError := call(t, session, "knowledge_sources", map[string]any{"page": "features/share.md"}, &out)
	if isError {
		t.Fatal(text)
	}
	want := []Source{
		{Pointer: "notes-app:src/share.go:3", Status: "found", PageLine: 13},
		{Pointer: "notes-app:src/gone.go:1", Status: "missing", PageLine: 13},
		{Pointer: "notes-api:src/link.go:9", Status: "unchecked", PageLine: 13},
	}
	if out.Page != "features/share.md" || !reflect.DeepEqual(out.Sources, want) {
		t.Errorf("sources %+v", out)
	}
	if !strings.Contains(text, "notes-app:src/gone.go:1  missing") {
		t.Errorf("text %q", text)
	}
}

func TestPagePathsOutsideTheCloneAreRefused(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	for _, tool := range []string{"knowledge_read", "knowledge_sources"} {
		for _, page := range []string{"../secret.md", "features/../../secret.md", filepath.Join(filepath.Dir(clone), "secret.md"), ".git/HEAD"} {
			text, isError := call(t, session, tool, map[string]any{"page": page}, nil)
			if !isError || strings.Contains(text, "secret\n") {
				t.Errorf("%s %q: %v %q", tool, page, isError, text)
			}
		}
	}
}

func TestTheIndexIsRebuiltWhenTheCloneHeadChanges(t *testing.T) {
	clone, project := fixture(t)
	session := connect(t, clone, project)
	search := func() []SearchResult {
		var out SearchOutput
		call(t, session, "knowledge_search", map[string]any{"query": "invoice"}, &out)
		return out.Results
	}
	if len(search()) != 0 {
		t.Fatal("invoice found before it was written")
	}
	write(t, clone, "features/invoice.md", "---\ntitle: Invoice\n---\n\nAn invoice is a PDF.\n")
	if len(search()) != 0 {
		t.Error("index rebuilt without a HEAD change")
	}
	write(t, clone, ".git/HEAD", shaTwo+"\n")
	if got := search(); len(got) != 1 || got[0].Page != "features/invoice.md" {
		t.Errorf("after the HEAD change %+v", got)
	}
}

func TestNewRefusesAMissingClone(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "none"), t.TempDir()); err == nil {
		t.Error("missing clone accepted")
	}
}

func TestKnowledgeSearchGivesTheKindAndFiltersByIt(t *testing.T) {
	clone, project := fixture(t)
	write(t, clone, "specs/1.4.0/share-expiry/prd.md", "---\ntitle: Share expiry\nfeature: share-expiry\nstatus: approved\nrev: 1\n"+
		"repos: [notes-app]\n---\n\n- AC-1 Given a share link older than a week, when it opens, then it says the link expired.\n")
	session := connect(t, clone, project)
	var out SearchOutput
	call(t, session, "knowledge_search", map[string]any{"query": "share link expired week"}, &out)
	kinds := map[string]string{}
	for _, result := range out.Results {
		kinds[result.Page] = result.Kind
	}
	if kinds["features/share.md"] != "feature" || kinds["specs/1.4.0/share-expiry/prd.md"] != "spec" {
		t.Errorf("kinds %v", kinds)
	}
	text, _ := call(t, session, "knowledge_search", map[string]any{"query": "share link expired week", "kind": "spec"}, &out)
	if len(out.Results) != 1 || out.Results[0].Page != "specs/1.4.0/share-expiry/prd.md" || !strings.Contains(text, "[spec]") {
		t.Errorf("spec filter %+v %q", out.Results, text)
	}
	call(t, session, "knowledge_search", map[string]any{"query": "share", "kind": "repo"}, &out)
	if len(out.Results) != 0 {
		t.Errorf("repo filter %+v", out.Results)
	}
	if text, isError := call(t, session, "knowledge_search", map[string]any{"query": "share", "kind": "page"}, nil); !isError || !strings.Contains(text, "spec, feature, repo or other") {
		t.Errorf("unknown kind: %v %q", isError, text)
	}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name == "knowledge_search" && !strings.Contains(tool.Description, "kind") {
			t.Errorf("description %q", tool.Description)
		}
	}
}
