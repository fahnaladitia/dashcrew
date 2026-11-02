package link

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

const flutterProfile = "toolkit: 1\nagents: [claude-code]\nproject:\n  links:\n    - { package: shared_core }\n"

const goProfile = "toolkit: 1\nagents: [claude-code]\nplatform: other\n" +
	"platform_config: { detect: [go.mod], commands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }, link: go.work }\n" +
	"project:\n  links:\n    - { package: github.com/acme/api }\n"

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// workspace makes an app checkout and a sibling package folder; it returns the app root and the package folder.
func workspace(t *testing.T, profileText, appManifest, manifestName, packageManifest string) (string, string) {
	t.Helper()
	root := t.TempDir()
	app := filepath.Join(root, "app")
	pkg := filepath.Join(root, "core")
	write(t, filepath.Join(app, ".ai", "project.yaml"), profileText)
	write(t, filepath.Join(app, manifestName), appManifest)
	write(t, filepath.Join(pkg, manifestName), packageManifest)
	run(t, app, "init", "-q")
	return app, pkg
}

func open(t *testing.T, dir string) *Links {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	l, err := Open(dir, c)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func refusal(t *testing.T, err error, want string) {
	t.Helper()
	var r *Error
	if !errors.As(err, &r) || !strings.Contains(r.Message, want) {
		t.Errorf("got %v, want a refusal containing %q", err, want)
	}
}

func TestAddWritesPubspecOverridesAndExcludesIt(t *testing.T) {
	app, pkg := workspace(t, flutterProfile, "name: app\n", "pubspec.yaml", "name: shared_core\n")
	l := open(t, filepath.Join(app, ".ai"))
	entries, err := l.Add("shared_core", pkg, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Entry{{"shared_core", "../core"}}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries %#v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(app, "pubspec_overrides.yaml"))
	want := "# " + Header + "\ndependency_overrides:\n  shared_core:\n    path: \"../core\"\n"
	if string(data) != want {
		t.Errorf("file\n%s", data)
	}
	exclude, _ := os.ReadFile(filepath.Join(app, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "\n/pubspec_overrides.yaml\n") && !strings.HasPrefix(string(exclude), "/pubspec_overrides.yaml\n") {
		t.Errorf("exclude\n%s", exclude)
	}
	if read, err := l.Read(); err != nil || !reflect.DeepEqual(read, entries) {
		t.Errorf("read back %#v %v", read, err)
	}
	if _, err := l.Add("shared_core", pkg, false); err != nil {
		t.Errorf("adding again: %v", err)
	}
	if entries, err := l.Remove("shared_core"); err != nil || len(entries) != 0 {
		t.Errorf("remove %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(app, "pubspec_overrides.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the empty file was kept")
	}
}

func TestAddWritesGoWork(t *testing.T) {
	app, pkg := workspace(t, goProfile, "module github.com/acme/app\n\ngo 1.26\n", "go.mod", "module github.com/acme/api\n\ngo 1.26\n")
	l := open(t, app)
	if _, err := l.Add("github.com/acme/api", pkg, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(app, "go.work"))
	want := "// " + Header + "\ngo 1.26\n\nuse .\nuse ../core // github.com/acme/api\n"
	if string(data) != want {
		t.Errorf("go.work\n%s", data)
	}
	exclude, _ := os.ReadFile(filepath.Join(app, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "/go.work\n/go.work.sum\n") {
		t.Errorf("exclude\n%s", exclude)
	}
	if read, _ := l.Read(); !reflect.DeepEqual(read, []Entry{{"github.com/acme/api", "../core"}}) {
		t.Errorf("read back %#v", read)
	}
}

func TestAddRefusals(t *testing.T) {
	app, pkg := workspace(t, flutterProfile, "name: app\n", "pubspec.yaml", "name: other_pkg\n")
	l := open(t, app)

	_, err := l.Add("unlisted", pkg, false)
	refusal(t, err, "unlisted is not in project.links")

	_, err = l.Add("shared_core", pkg, false)
	refusal(t, err, `holds "other_pkg", not "shared_core"`)

	_, err = l.Add("shared_core", filepath.Join(pkg, "missing"), false)
	refusal(t, err, "is not a folder")

	write(t, filepath.Join(pkg, "pubspec.yaml"), "name: shared_core\n")
	write(t, filepath.Join(app, "pubspec_overrides.yaml"), "dependency_overrides:\n  x: { path: ../x }\n")
	_, err = l.Add("shared_core", pkg, false)
	refusal(t, err, "was not written by dashcrew link")
	if _, err := l.Add("shared_core", pkg, true); err != nil {
		t.Errorf("--force: %v", err)
	}

	run(t, app, "add", "-f", "pubspec_overrides.yaml")
	_, err = l.Add("shared_core", pkg, false)
	refusal(t, err, "is committed in this repository")
}

func TestNoLinkFile(t *testing.T) {
	profileText := "toolkit: 1\nagents: [claude-code]\nplatform: other\n" +
		"platform_config: { detect: [x], commands: { analyze: a, test: b, test_scoped: c, deps: d } }\n" +
		"project:\n  links:\n    - { package: core }\n"
	app, pkg := workspace(t, profileText, "", "x", "")
	_, err := open(t, app).Add("core", pkg, false)
	refusal(t, err, "platform other has no link file; set platform_config.link to pubspec_overrides.yaml or go.work")
}

func TestRemoveRefusesAnUnlinkedPackage(t *testing.T) {
	app, _ := workspace(t, flutterProfile, "name: app\n", "pubspec.yaml", "name: shared_core\n")
	_, err := open(t, app).Remove("shared_core")
	refusal(t, err, "shared_core is not linked")
}
