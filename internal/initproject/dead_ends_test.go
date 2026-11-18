package initproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func goModule(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	write(t, project, "go.mod", "module example.com/app\n\ngo 1.22\n\nrequire (\n\tgithub.com/go-chi/chi/v5 v5.0.0\n\tgithub.com/jackc/pgx/v5 v5.7.4\n)\n")
	return project
}

func TestAGoModuleGetsTheBuiltInGoPlatformWithYes(t *testing.T) {
	project := goModule(t)

	result, err := runInit(t, project, initRun{yes: true})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	for _, line := range []string{"platform: go\n", "http: chi\n", "db: pgx\n"} {
		if !strings.Contains(result.profile, line) {
			t.Errorf("profile lacks %q:\n%s", line, result.profile)
		}
	}
	if _, err := os.Stat(filepath.Join(project, ".ai", "platforms")); err == nil {
		t.Error("init wrote a platform folder for the built-in go platform")
	}
	for _, line := range []string{"  platform: go (go.mod)", "  db: pgx (github.com/jackc/pgx/v5 in go.mod)", "  http: chi (github.com/go-chi/chi/v5 in go.mod)", "next: dashcrew sync"} {
		if !has(result.output, line) {
			t.Errorf("output lacks %q: %v", line, result.output)
		}
	}
	if _, err := profile.Parse(result.profile, shippedCatalog(t).KnownOptions()); err != nil {
		t.Errorf("the profile does not parse: %v", err)
	}
}

func TestAGoModuleIsProposedGoAndAsksItsFamilies(t *testing.T) {
	project := goModule(t)

	result, err := runInit(t, project, initRun{answers: []string{"", "", "", "gin"}})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	if !has(result.output, "platform [go]: ") || !has(result.output, "db [pgx]: ") || !has(result.output, "http [chi]: ") {
		t.Errorf("output %v", result.output)
	}
	if !strings.Contains(result.profile, "http: gin\n") || !strings.Contains(result.profile, "db: pgx\n") {
		t.Errorf("profile:\n%s\noutput %q", result.profile, result.output)
	}
}

func TestAGoModuleMayStillChooseOther(t *testing.T) {
	project := goModule(t)

	result, err := runInit(t, project, initRun{answers: []string{"other", "", "go vet ./...", "go test ./...", "go test", "go mod download", ""}})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	if !strings.Contains(result.profile, "platform: other\n") || !strings.Contains(result.profile, "detect: [go.mod]") {
		t.Errorf("profile:\n%s", result.profile)
	}
}

// TestOtherProposesTheMarkerFilesItFound pins that the detect files are printed before any prompt and proposed as
// the default answer.
func TestOtherProposesTheMarkerFilesItFound(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pyproject.toml", "")
	write(t, project, "Makefile", "")

	result, err := runInit(t, project, initRun{answers: []string{"", "", "ruff check .", "pytest", "pytest", "pip install -e .", ""}})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	files := indexOf(result.output, "  files: pyproject.toml, Makefile")
	prompt := indexOf(result.output, "platform [other]: ")
	if files < 0 || prompt < 0 || files > prompt {
		t.Errorf("files not printed before the first prompt: %v", result.output)
	}
	if !has(result.output, "detect (files that identify the platform, space separated) [pyproject.toml Makefile]: ") {
		t.Errorf("no proposed detect files in %v", result.output)
	}
	parsed, err := profile.Parse(result.profile, shippedCatalog(t).KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(parsed.PlatformConfig.Detect, " "); got != "pyproject.toml Makefile" {
		t.Errorf("detect %q", got)
	}
}

func TestARequiredAnswerLeftEmptyTwiceStops(t *testing.T) {
	project := t.TempDir()

	result, err := runInit(t, project, initRun{answers: []string{"", "", "", "x", "x", "x", "x", "x"}})

	initErrorContaining(t, err, "needs detect")
	if n := count(result.output, "  required"); n != 1 {
		t.Errorf("asked %d times more, want once: %v", n, result.output)
	}
	if _, statErr := os.Stat(filepath.Join(project, ".ai", "project.yaml")); statErr == nil {
		t.Error("a profile was written")
	}
}

func indexOf(lines []string, line string) int {
	for i, l := range lines {
		if l == line {
			return i
		}
	}
	return -1
}
