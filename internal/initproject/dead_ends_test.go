package initproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// runWithStarters is runInit with the shipped starters offered, as the CLI runs it.
func runWithStarters(t *testing.T, project string, r initRun) (initResult, error) {
	t.Helper()
	output := []string{}
	pending := append([]string{}, r.answers...)
	err := Init{Catalog: shippedCatalog(t), ProjectDir: project, Starters: embedded.StartersFS, Version: "0.8.0"}.Run(r.yes, r.force, nil, func(prompt string) *string {
		output = append(output, prompt)
		if len(pending) == 0 {
			return nil
		}
		answer := pending[0]
		pending = pending[1:]
		return &answer
	}, func(line string) { output = append(output, line) })
	text, _ := os.ReadFile(filepath.Join(project, ".ai", "project.yaml"))
	return initResult{string(text), output}, err
}

func goModule(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	write(t, project, "go.mod", "module example.com/app\n\ngo 1.22\n\nrequire github.com/go-chi/chi/v5 v5.0.0\n")
	return project
}

func TestAGoModuleGetsTheGoStarterWithYes(t *testing.T) {
	project := goModule(t)

	result, err := runWithStarters(t, project, initRun{yes: true})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	if !strings.Contains(result.profile, "platform: go\n") || !strings.Contains(result.profile, "http: chi\n") {
		t.Errorf("profile:\n%s", result.profile)
	}
	if _, err := os.Stat(filepath.Join(project, ".ai", "platforms", "go", "platform.yaml")); err != nil {
		t.Errorf("starter not written: %v", err)
	}
	if !has(result.output, "  starter: go (go.mod)") || has(result.output, "  tip: this is a Go module; dashcrew platform new go writes a Go platform for this repo to edit, then run dashcrew sync") {
		t.Errorf("output %v", result.output)
	}
	if !has(result.output, "next: dashcrew sync") {
		t.Errorf("no next step in %v", result.output)
	}
}

func TestAGoModuleIsOfferedTheGoStarterFirst(t *testing.T) {
	project := goModule(t)

	result, err := runWithStarters(t, project, initRun{answers: []string{""}})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	if !has(result.output, "platform [go]: ") || !strings.Contains(result.profile, "platform: go\n") {
		t.Errorf("output %v\nprofile:\n%s", result.output, result.profile)
	}
}

func TestAGoModuleMayStillChooseOther(t *testing.T) {
	project := goModule(t)

	result, err := runWithStarters(t, project, initRun{answers: []string{"other", "", "go vet ./...", "go test ./...", "go test", "go mod download", ""}})

	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(result.output, "\n"))
	}
	if !strings.Contains(result.profile, "platform: other\n") || !strings.Contains(result.profile, "detect: [go.mod]") {
		t.Errorf("profile:\n%s", result.profile)
	}
	if _, err := os.Stat(filepath.Join(project, ".ai", "platforms")); err == nil {
		t.Error("a starter was written for platform: other")
	}
}

// TestOtherProposesTheMarkerFilesItFound pins that the detect files are printed before any prompt and proposed as
// the default answer.
func TestOtherProposesTheMarkerFilesItFound(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pyproject.toml", "")
	write(t, project, "Makefile", "")

	result, err := runWithStarters(t, project, initRun{answers: []string{"", "", "ruff check .", "pytest", "pytest", "pip install -e .", ""}})

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

	result, err := runWithStarters(t, project, initRun{answers: []string{"", "", "", "x", "x", "x", "x", "x"}})

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
