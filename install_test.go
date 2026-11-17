package dashcrew

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease is a release server for install.sh: `/latest` redirects to the tag of latest, and each tag in
// binaries serves an archive for this machine whose `dashcrew` prints the given `--version` output.
type fakeRelease struct {
	latest   string
	binaries map[string]string
	// checksums overrides checksums.txt per tag; "-" serves none.
	checksums map[string]string
}

func (f fakeRelease) archiveName(version string) string {
	return "dashcrew_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
}

func (f fakeRelease) serve(t *testing.T) string {
	t.Helper()
	archives := map[string][]byte{}
	for tag, versionOutput := range f.binaries {
		archives[tag] = tarGz(t, map[string]string{
			"LICENSE":   "license\n",
			"README.md": "readme\n",
			"dashcrew":  "#!/bin/sh\necho '" + versionOutput + "'\n",
		})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+f.latest, http.StatusFound)
	})
	mux.HandleFunc("/releases/tag/", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		tag, file, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/releases/download/"), "/")
		archive, ok := archives[tag]
		if !ok {
			http.NotFound(w, r)
			return
		}
		version := strings.TrimPrefix(tag, "v")
		switch file {
		case f.archiveName(version):
			w.Write(archive)
		case "checksums.txt":
			sums, overridden := f.checksums[tag]
			if !overridden {
				sum := sha256.Sum256(archive)
				sums = hex.EncodeToString(sum[:]) + "  " + f.archiveName(version) + "\n"
			}
			if sums == "-" {
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL + "/releases"
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"LICENSE", "README.md", "dashcrew"} {
		mode := int64(0o644)
		if name == "dashcrew" {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(files[name]))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// runInstall runs install.sh with the given environment on top of a minimal one and returns its combined output
// and exit error.
func runInstall(t *testing.T, releases, installDir string, env ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"DASHCREW_RELEASES_URL=" + releases,
		"DASHCREW_INSTALL_DIR=" + installDir,
	}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func installedVersion(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command(filepath.Join(dir, "dashcrew"), "--version").Output()
	if err != nil {
		t.Fatalf("installed dashcrew: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func skipWithoutPOSIX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh needs a POSIX shell; Windows has no install script yet")
	}
}

func TestInstallScriptInstallsTheLatestRelease(t *testing.T) {
	skipWithoutPOSIX(t)
	releases := fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}}.serve(t)
	dir := filepath.Join(t.TempDir(), "bin")

	out, err := runInstall(t, releases, dir)

	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := installedVersion(t, dir); got != "dashcrew 9.9.9" {
		t.Errorf("installed %q", got)
	}
	if !strings.Contains(out, "dashcrew 9.9.9") || !strings.Contains(out, filepath.Join(dir, "dashcrew")) {
		t.Errorf("output does not name the version and path:\n%s", out)
	}
	if !strings.Contains(out, `export PATH="`+dir+`:$PATH"`) {
		t.Errorf("output does not say how to put %s on PATH:\n%s", dir, out)
	}
}

func TestInstallScriptInstallsAPinnedVersion(t *testing.T) {
	skipWithoutPOSIX(t)
	releases := fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9", "v9.9.8": "dashcrew 9.9.8"}}.serve(t)
	for _, pinned := range []string{"v9.9.8", "9.9.8"} {
		dir := filepath.Join(t.TempDir(), "bin")

		out, err := runInstall(t, releases, dir, "DASHCREW_VERSION="+pinned)

		if err != nil {
			t.Fatalf("%s: %v\n%s", pinned, err, out)
		}
		if got := installedVersion(t, dir); got != "dashcrew 9.9.8" {
			t.Errorf("%s: installed %q", pinned, got)
		}
	}
}

func TestInstallScriptReplacesAnInstalledDashcrew(t *testing.T) {
	skipWithoutPOSIX(t)
	releases := fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}}.serve(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dashcrew"), []byte("#!/bin/sh\necho 'dashcrew 0.1.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runInstall(t, releases, dir, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := installedVersion(t, dir); got != "dashcrew 9.9.9" {
		t.Errorf("installed %q", got)
	}
	if strings.Contains(out, "export PATH") {
		t.Errorf("asks to add a folder already on PATH:\n%s", out)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("install folder holds %d entries, want only dashcrew", len(entries))
	}
}

func TestInstallScriptNamesAnEarlierDashcrewOnPath(t *testing.T) {
	skipWithoutPOSIX(t)
	releases := fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}}.serve(t)
	earlier := t.TempDir()
	if err := os.WriteFile(filepath.Join(earlier, "dashcrew"), []byte("#!/bin/sh\necho 'dashcrew 0.1.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "bin")

	out, err := runInstall(t, releases, dir, "PATH="+earlier+string(os.PathListSeparator)+dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, filepath.Join(earlier, "dashcrew")) {
		t.Errorf("output does not name the dashcrew that comes first on PATH:\n%s", out)
	}
}

// TestInstallScriptRefusesWhatItCannotVerify pins that nothing is installed unless the archive's checksum is listed
// and matches, and the binary reports the version asked for.
func TestInstallScriptRefusesWhatItCannotVerify(t *testing.T) {
	skipWithoutPOSIX(t)
	archive := fakeRelease{}.archiveName("9.9.9")
	for _, c := range []struct {
		name    string
		release fakeRelease
		env     []string
		message string
	}{
		{"wrong hash", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"},
			checksums: map[string]string{"v9.9.9": strings.Repeat("0", 64) + "  " + archive + "\n"}}, nil, "checksum"},
		{"archive not listed", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"},
			checksums: map[string]string{"v9.9.9": strings.Repeat("0", 64) + "  other.tar.gz\n"}}, nil, "checksums.txt"},
		{"no checksums.txt", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"},
			checksums: map[string]string{"v9.9.9": "-"}}, nil, "checksums.txt"},
		{"another version inside", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 1.0.0"}}, nil, "9.9.9"},
		{"no such release", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}},
			[]string{"DASHCREW_VERSION=7.7.7"}, "7.7.7"},
		{"a version that is not one", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}},
			[]string{"DASHCREW_VERSION=latest; rm -rf /"}, "not a version"},
		{"a version with a second line", fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}},
			[]string{"DASHCREW_VERSION=9.9.9\nx"}, "not a version"},
	} {
		t.Run(c.name, func(t *testing.T) {
			releases := c.release.serve(t)
			dir := filepath.Join(t.TempDir(), "bin")

			out, err := runInstall(t, releases, dir, c.env...)

			if err == nil {
				t.Fatalf("installed:\n%s", out)
			}
			if !strings.Contains(out, c.message) {
				t.Errorf("output does not mention %q:\n%s", c.message, out)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("install folder holds %d entries after a refusal", len(entries))
			}
		})
	}
}

// TestInstallScriptWorksWithWgetAlone runs install.sh on a PATH with wget and without curl.
func TestInstallScriptWorksWithWgetAlone(t *testing.T) {
	skipWithoutPOSIX(t)
	wget, err := exec.LookPath("wget")
	if err != nil {
		t.Skip("wget is not installed")
	}
	bin := t.TempDir()
	tools := map[string]string{"wget": wget}
	for _, name := range []string{"sh", "uname", "grep", "mktemp", "rm", "awk", "sed", "tail", "tr", "mkdir", "tar", "gzip", "cp", "chmod", "mv", "sha256sum", "shasum"} {
		if path, err := exec.LookPath(name); err == nil {
			tools[name] = path
		}
	}
	for name, path := range tools {
		if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	releases := fakeRelease{latest: "v9.9.9", binaries: map[string]string{"v9.9.9": "dashcrew 9.9.9"}}.serve(t)
	dir := filepath.Join(t.TempDir(), "bin")

	out, err := runInstall(t, releases, dir, "PATH="+bin)

	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := installedVersion(t, dir); got != "dashcrew 9.9.9" {
		t.Errorf("installed %q", got)
	}
}
