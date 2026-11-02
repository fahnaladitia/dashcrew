package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLinksCheckOfAPullRequest(t *testing.T) {
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(t.TempDir(), "core")
	gitRun(t, filepath.Dir(core), "init", "-q", "-b", "main", core)
	gitRun(t, core, "commit", "-q", "--allow-empty", "-m", "a")
	gitRun(t, core, "tag", "v1.0.0")
	gitRun(t, core, "branch", "feat/x")

	p := &profile.Profile{Platform: profile.DefaultPlatform, Links: []profile.Link{{Package: "shared_core"}}}
	for ref, want := range map[string]doctor.Status{"v1.0.0": doctor.OK, "feat/x": doctor.Fail} {
		dir := t.TempDir()
		gitRun(t, dir, "init", "-q")
		pubspec := "name: app\ndependencies:\n  shared_core:\n    git:\n      url: " + core + "\n      ref: " + ref + "\n"
		if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatal(err)
		}
		got := PrCheck{Catalog: c, ProjectDir: dir}.links(p)
		if got.ID != "links" || got.Status != want {
			t.Errorf("ref %s: %+v", ref, got)
		}
	}

	other := &profile.Profile{Platform: profile.OtherOption, PlatformConfig: &profile.PlatformConfig{}, Links: p.Links}
	if got := (PrCheck{Catalog: c, ProjectDir: t.TempDir()}).links(other); got.Status != doctor.Fail || !strings.Contains(got.Detail, "has no link file") {
		t.Errorf("no link file: %+v", got)
	}
}
