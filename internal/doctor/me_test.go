package doctor

import (
	"io/fs"
	"os"
	"os/exec"
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
	expect(OK, "role mobile (home), level regular (home), stacks flutter; this project's platform go is outside your stacks")

	if err := (&me.RepoProfile{Role: "lead"}).Save(project); err != nil {
		t.Fatal(err)
	}
	if err := me.Register(places, project); err != nil {
		t.Fatal(err)
	}
	if _, err := me.Sync(p, places, personal); err != nil {
		t.Fatal(err)
	}
	expect(OK, "role lead (repo), level regular (home)")

	for _, args := range [][]string{{"init", "-q"}, {"add", "-f", ".ai/me.yaml"}} {
		if out, err := exec.Command("git", append([]string{"-C", project}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	expect(Fail, "personal files tracked by git: .ai/me.yaml")
}
