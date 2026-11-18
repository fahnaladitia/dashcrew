package starter

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
	if got := Names(embedded.StartersFS); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("names %v", got)
	}
	if got := Options(embedded.StartersFS, "go", "http"); !reflect.DeepEqual(got, []string{"chi", "echo", "fiber", "gin", "nethttp", "none"}) {
		t.Errorf("http %v", got)
	}
	if got := Options(embedded.StartersFS, "go", "db"); !reflect.DeepEqual(got, []string{"gorm", "none", "pgx", "sqlc"}) {
		t.Errorf("db %v", got)
	}
}

func TestWriteOnceWithTheVersion(t *testing.T) {
	project := t.TempDir()
	written, err := Write(embedded.StartersFS, "go", project, "9.8.7")
	if err != nil {
		t.Fatal(err)
	}
	if len(written) < 20 || !strings.HasPrefix(written[0], ".ai/platforms/go/") {
		t.Errorf("written %v", written)
	}
	manifest, _ := os.ReadFile(filepath.Join(project, ".ai", "platforms", "go", "platform.yaml"))
	if !strings.Contains(string(manifest), "Starter from dashcrew 9.8.7") || strings.Contains(string(manifest), "{{version}}") {
		t.Errorf("platform.yaml header:\n%s", manifest)
	}
	if _, err := Write(embedded.StartersFS, "go", project, "9.8.7"); err == nil || !strings.Contains(err.Error(), "never overwrites") {
		t.Errorf("second write: %v", err)
	}
	if _, err := Write(embedded.StartersFS, "rust", t.TempDir(), "1"); err == nil || !strings.Contains(err.Error(), "available: go") {
		t.Errorf("unknown starter: %v", err)
	}
}

func TestGoDefaults(t *testing.T) {
	cases := []struct{ gomod, extra, http, db string }{
		{"", "", "nethttp", "none"},
		{"require github.com/go-chi/chi/v5 v5.2.1\nrequire github.com/jackc/pgx/v5 v5.7.4\n", "", "chi", "pgx"},
		{"require (\n\tgithub.com/gin-gonic/gin v1.10.0\n\tgorm.io/gorm v1.25.12\n)\n", "", "gin", "gorm"},
		{"require (\n\tgithub.com/labstack/echo/v4 v4.13.3\n\tgithub.com/jackc/pgx/v5 v5.7.4\n)\n", "sqlc.yaml", "echo", "sqlc"},
		{"require github.com/gofiber/fiber/v2 v2.52.6\n", "", "fiber", "none"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if c.gomod != "" {
			os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.26\n\n"+c.gomod), 0o644)
		}
		if c.extra != "" {
			os.WriteFile(filepath.Join(dir, c.extra), []byte("version: \"2\"\n"), 0o644)
		}
		if http, db := GoDefaults(dir); http != c.http || db != c.db {
			t.Errorf("%q: %s %s, want %s %s", c.gomod, http, db, c.http, c.db)
		}
	}
}

// TestEveryGoPackSyncs syncs the starter with every http and db option, for the three agents: it must load, sync,
// pass doctor and keep AGENTS.md under Antigravity's limit.
func TestEveryGoPackSyncs(t *testing.T) {
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, http := range Options(embedded.StartersFS, "go", "http") {
		for _, db := range Options(embedded.StartersFS, "go", "db") {
			project := t.TempDir()
			if _, err := Write(embedded.StartersFS, "go", project, "0.0.0"); err != nil {
				t.Fatal(err)
			}
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
