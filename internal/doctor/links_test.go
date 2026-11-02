package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/link"
)

// linkedProject is an app checkout on feat/a whose profile lists shared_core, next to a core checkout on coreBranch.
func linkedProject(t *testing.T, coreBranch string) (Doctor, string, string) {
	t.Helper()
	root := t.TempDir()
	app, core := filepath.Join(root, "app"), filepath.Join(root, "core")
	writeFile(t, filepath.Join(app, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\nproject:\n  links:\n    - { package: shared_core }\n")
	writeFile(t, filepath.Join(app, "pubspec.yaml"), "name: app\n")
	writeFile(t, filepath.Join(core, "pubspec.yaml"), "name: shared_core\n")
	gitIn(t, app, "init", "-q")
	gitIn(t, app, "checkout", "-q", "-b", "feat/a")
	gitIn(t, core, "init", "-q")
	gitIn(t, core, "checkout", "-q", "-b", coreBranch)
	return Doctor{Catalog: shippedCatalog, ProjectDir: app}, app, core
}

func linksCheck(t *testing.T, d Doctor) Check {
	t.Helper()
	check, ok := d.links()
	if !ok {
		t.Fatal("no links check")
	}
	return check
}

func addLink(t *testing.T, app, core string) {
	t.Helper()
	l, err := link.Open(app, shippedCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Add("shared_core", core, false); err != nil {
		t.Fatal(err)
	}
}

func TestLinksCheck(t *testing.T) {
	setup(t, blocProfile)
	t.Run("absent without links and without a file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
		if _, ok := (Doctor{Catalog: shippedCatalog, ProjectDir: root}).links(); ok {
			t.Error("a links check without links")
		}
	})
	t.Run("ok with nothing linked yet", func(t *testing.T) {
		d, _, _ := linkedProject(t, "feat/a")
		expectCheck(t, linksCheck(t, d), OK, "no links")
	})
	t.Run("ok on the same branch", func(t *testing.T) {
		d, app, core := linkedProject(t, "feat/a")
		addLink(t, app, core)
		expectCheck(t, linksCheck(t, d), OK, "1 linked in pubspec_overrides.yaml")
	})
	t.Run("info on another branch", func(t *testing.T) {
		d, app, core := linkedProject(t, "feat/a-core")
		addLink(t, app, core)
		expectCheck(t, linksCheck(t, d), Info, "this checkout is on feat/a; shared_core on feat/a-core")
	})
	t.Run("warn when the linked checkout is gone", func(t *testing.T) {
		d, app, core := linkedProject(t, "feat/a")
		addLink(t, app, core)
		if err := os.RemoveAll(core); err != nil {
			t.Fatal(err)
		}
		expectCheck(t, linksCheck(t, d), Warn, "linked checkout missing: shared_core (../core)")
	})
	t.Run("warn on a file git would commit", func(t *testing.T) {
		d, app, _ := linkedProject(t, "feat/a")
		writeFile(t, filepath.Join(app, "pubspec_overrides.yaml"), "dependency_overrides: {}\n")
		expectCheck(t, linksCheck(t, d), Warn, "is not ignored by git and would be committed")
	})
	t.Run("warn on an ignored file dashcrew did not write", func(t *testing.T) {
		d, app, _ := linkedProject(t, "feat/a")
		writeFile(t, filepath.Join(app, ".gitignore"), "pubspec_overrides.yaml\n")
		writeFile(t, filepath.Join(app, "pubspec_overrides.yaml"), "dependency_overrides: {}\n")
		expectCheck(t, linksCheck(t, d), Warn, "was not written by dashcrew link")
	})
	t.Run("fail on a committed file", func(t *testing.T) {
		d, app, _ := linkedProject(t, "feat/a")
		writeFile(t, filepath.Join(app, "pubspec_overrides.yaml"), "dependency_overrides: {}\n")
		gitIn(t, app, "add", "pubspec_overrides.yaml")
		expectCheck(t, linksCheck(t, d), Fail, "pubspec_overrides.yaml is committed")
	})
}

func expectCheck(t *testing.T, check Check, status Status, detail string) {
	t.Helper()
	if check.Status != status || !strings.Contains(check.Detail, detail) {
		t.Errorf("got %s %q, want %s containing %q", check.Status, check.Detail, status, detail)
	}
}
