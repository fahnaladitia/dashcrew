package eject

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

func TestNamesAndOptions(t *testing.T) {
	if got := Names(embedded.CatalogFS); !reflect.DeepEqual(got, []string{"flutter", "go"}) {
		t.Errorf("names %v", got)
	}
	if got := Options(embedded.CatalogFS, "go", "http"); !reflect.DeepEqual(got, []string{"chi", "echo", "fiber", "gin", "nethttp", "none"}) {
		t.Errorf("http %v", got)
	}
	if got := Options(embedded.CatalogFS, "go", "db"); !reflect.DeepEqual(got, []string{"gorm", "none", "pgx", "sqlc"}) {
		t.Errorf("db %v", got)
	}
}

func TestWriteOnceWithTheVersion(t *testing.T) {
	project := t.TempDir()
	written, err := Write(embedded.CatalogFS, "go", project, "9.8.7")
	if err != nil {
		t.Fatal(err)
	}
	if len(written) < 20 || !strings.HasPrefix(written[0], ".ai/platforms/go/") {
		t.Errorf("written %v", written)
	}
	manifest, _ := os.ReadFile(filepath.Join(project, ".ai", "platforms", "go", "platform.yaml"))
	if !strings.HasPrefix(string(manifest), "# Ejected from dashcrew 9.8.7: ") {
		t.Errorf("platform.yaml header:\n%s", manifest)
	}
	if _, err := Write(embedded.CatalogFS, "go", project, "9.8.7"); err == nil || !strings.Contains(err.Error(), "never overwrites") {
		t.Errorf("second write: %v", err)
	}
	if _, err := Write(embedded.CatalogFS, "rust", t.TempDir(), "1"); err == nil || !strings.Contains(err.Error(), "available: flutter, go") {
		t.Errorf("unknown platform: %v", err)
	}
}

// TestAnEjectedPlatformRendersLikeTheBuiltIn: ejecting changes who owns the rules, not what agents read.
func TestAnEjectedPlatformRendersLikeTheBuiltIn(t *testing.T) {
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for name, profile := range map[string]string{
		"go":      "toolkit: 1\nplatform: go\nagents: [claude-code]\nstack:\n  http: chi\n  db: pgx\n",
		"flutter": "toolkit: 1\nagents: [claude-code]\nstack:\n  state: bloc\n",
	} {
		builtIn, ejected := t.TempDir(), t.TempDir()
		if _, err := Write(embedded.CatalogFS, name, ejected, "0.0.0"); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{builtIn, ejected} {
			os.MkdirAll(filepath.Join(dir, ".ai"), 0o755)
			os.WriteFile(filepath.Join(dir, ".ai", "project.yaml"), []byte(profile), 0o644)
			if _, err := (sync.Sync{Catalog: c, ProjectDir: dir, Home: t.TempDir()}).Run(false, false); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		for _, file := range []string{"AGENTS.md", ".claude/skills/plan/SKILL.md", ".ai/guard.json"} {
			a, _ := os.ReadFile(filepath.Join(builtIn, file))
			b, _ := os.ReadFile(filepath.Join(ejected, file))
			if string(a) != string(b) {
				t.Errorf("%s: %s differs between the built-in and the ejected platform", name, file)
			}
		}
	}
}

// TestEveryGoPackSyncs syncs the built-in go platform with every http and db option, for the three agents: it must
// sync, pass doctor and keep AGENTS.md under Antigravity's limit.
func TestEveryGoPackSyncs(t *testing.T) {
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, http := range Options(embedded.CatalogFS, "go", "http") {
		for _, db := range Options(embedded.CatalogFS, "go", "db") {
			project := t.TempDir()
			os.MkdirAll(filepath.Join(project, ".ai"), 0o755)
			profile := "toolkit: 1\nplatform: go\nagents: [claude-code, codex, antigravity]\nstack:\n  http: " + http + "\n  db: " + db + "\n"
			os.WriteFile(filepath.Join(project, ".ai", "project.yaml"), []byte(profile), 0o644)
			if _, err := (sync.Sync{Catalog: c, ProjectDir: project, Home: t.TempDir()}).Run(false, false); err != nil {
				t.Fatalf("%s %s: %v", http, db, err)
			}
			agents, _ := os.ReadFile(filepath.Join(project, "AGENTS.md"))
			if len(agents) > doctor.AgentsMdLimitFor(map[string]bool{"antigravity": true}) {
				t.Errorf("%s %s: AGENTS.md %d bytes", http, db, len(agents))
			}
			for _, check := range (doctor.Doctor{Catalog: c, ProjectDir: project, PathEnv: t.TempDir(), MachineChecks: true}).Run() {
				if check.Status == doctor.Fail && check.ID != "guard-binary" {
					t.Errorf("%s %s: doctor %s: %s", http, db, check.ID, check.Detail)
				}
			}
		}
	}
}
