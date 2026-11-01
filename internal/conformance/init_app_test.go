package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/initproject"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// initApp runs `init --yes` on test/golden/init-app/input with origin/HEAD, origin/develop and origin/main as
// the remote branches.
func initApp(t *testing.T) (*catalog.Catalog, string) {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	copyInto(t, tree(t, filepath.Join(goldenDir(t), "init-app", "input")), project)
	err = initproject.Init{Catalog: c, ProjectDir: project}.Run(true, false,
		[]string{"origin/HEAD", "origin/develop", "origin/main"},
		func(string) *string { return nil }, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	return c, project
}

func TestInitAppWritesExpectedProjectYaml(t *testing.T) {
	_, project := initApp(t)

	actual, err := os.ReadFile(filepath.Join(project, ".ai", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	expectedPath := filepath.Join(goldenDir(t), "init-app", "expected-project.yaml")
	if update {
		if err := os.WriteFile(expectedPath, actual, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Errorf("project.yaml differs from expected-project.yaml (%s):\n%s", updateHint, actual)
	}
}

func TestInitAppProfileSyncsAndPassesDoctorBesidesTheMissingDashcrew(t *testing.T) {
	c, project := initApp(t)
	if _, err := (sync.Sync{Catalog: c, ProjectDir: project}).Run(false, false); err != nil {
		t.Fatal(err)
	}

	for _, check := range (doctor.Doctor{Catalog: c, ProjectDir: project, PathEnv: t.TempDir(), MachineChecks: true}).Run() {
		if check.Status == doctor.Fail && check.ID != "guard-binary" {
			t.Errorf("doctor %s: %s", check.ID, check.Detail)
		}
	}
}
