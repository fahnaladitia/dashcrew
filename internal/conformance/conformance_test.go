package conformance

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// samples are the golden fixtures under test/golden/ that a sync from project.yaml reproduces. init-app and
// sample-migrate-p3 start from other inputs and have their own tests.
var samples = []string{
	"sample-bloc-getx",
	"sample-project-pack",
	"sample-riverpod-gorouter",
	"sample-codex",
	"sample-antigravity",
	"sample-other",
	"sample-project-platform",
}

func goldenDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "test", "golden"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// tree maps every file under root (slash-separated relative path) to its content; empty when root is missing.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(root); err != nil {
		return out
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		out[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func copyInto(t *testing.T, files map[string]string, root string) {
	t.Helper()
	for path, content := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestGoldenSamplesAreByteIdentical(t *testing.T) {
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	golden := goldenDir(t)
	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			fixture := filepath.Join(golden, sample)
			project := t.TempDir()
			profileYaml, err := os.ReadFile(filepath.Join(fixture, "project.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			copyInto(t, map[string]string{".ai/project.yaml": string(profileYaml)}, project)
			for _, folder := range []string{"packs", "platforms"} {
				copyInto(t, tree(t, filepath.Join(fixture, folder)), filepath.Join(project, ".ai", folder))
			}

			s := sync.Sync{Catalog: c, ProjectDir: project}
			if _, err := s.Run(false, false); err != nil {
				t.Fatal(err)
			}

			actual := tree(t, project)
			delete(actual, ".ai/project.yaml")
			for path := range actual {
				if strings.HasPrefix(path, ".ai/packs/") || strings.HasPrefix(path, ".ai/platforms/") {
					delete(actual, path)
				}
			}
			expected := tree(t, filepath.Join(fixture, "expected"))

			identical := 0
			for _, path := range sortedKeys(expected) {
				got, ok := actual[path]
				switch {
				case !ok:
					t.Errorf("missing %s", path)
				case got != expected[path]:
					t.Errorf("differs: %s", path)
				default:
					identical++
				}
			}
			for _, path := range sortedKeys(actual) {
				if _, ok := expected[path]; !ok {
					t.Errorf("not in expected/: %s", path)
				}
			}
			t.Logf("%s: %d compared, %d identical", sample, len(expected), identical)

			again, err := s.Run(false, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(again.Written) != 0 || len(again.Removed) != 0 {
				t.Errorf("second sync wrote %v, removed %v", again.Written, again.Removed)
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
		})
	}
}
