package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/initproject"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// initApp runs `init --yes` on test/golden/init-app/input with the remote branches of test/golden/init_golden_test.dart.
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
	expected, err := os.ReadFile(filepath.Join(goldenDir(t), "init-app", "expected-project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Errorf("project.yaml differs from expected-project.yaml:\n%s", actual)
	}
}

func TestInitAppProfileSyncsAndPassesDoctorBesidesTheUncompiledGuard(t *testing.T) {
	c, project := initApp(t)
	if _, err := (sync.Sync{Catalog: c, ProjectDir: project}).Run(false, false); err != nil {
		t.Fatal(err)
	}
	hash, err := guard.SourceHash()
	if err != nil {
		t.Fatal(err)
	}

	for _, check := range (doctor.Doctor{Catalog: c, ProjectDir: project, GuardSource: hash, GuardHome: t.TempDir(), MachineChecks: true}).Run() {
		if check.Status == doctor.Fail && check.ID != "guard-binary" {
			t.Errorf("doctor %s: %s", check.ID, check.Detail)
		}
	}
}
