package doctor

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/me"
)

func TestMeCheck(t *testing.T) {
	places, err := me.PlacesFrom(map[string]string{"HOME": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".ai", "project.yaml"), []byte("toolkit: 1\nplatform: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := Doctor{ProjectDir: project, Me: &places}
	expect := func(status Status, detail string) {
		t.Helper()
		if got := d.me(); got.Status != status || !strings.Contains(got.Detail, detail) {
			t.Fatalf("got %v %q, want %v %q", got.Status, got.Detail, status, detail)
		}
	}
	expect(Skip, "dashcrew me init")

	if err := os.MkdirAll(filepath.Dir(me.Path(places.Home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(me.Path(places.Home), []byte("role: boss\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expect(Warn, "me.yaml role")

	p := &me.Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	if err := p.Save(places.Home); err != nil {
		t.Fatal(err)
	}
	expect(Warn, "missing  "+filepath.Join(places.ClaudeDir, "rules", "dashcrew-me.md"))

	personal, err := fs.Sub(embedded.PersonalFS, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := me.Sync(p, places, personal); err != nil {
		t.Fatal(err)
	}
	expect(OK, "role mobile, stacks flutter; this project's platform go is outside your stacks")
}
