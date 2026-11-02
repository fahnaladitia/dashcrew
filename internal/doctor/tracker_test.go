package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGithubTrackerCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in gh is a POSIX sh script")
	}
	setup(t, blocProfile)
	stub := func(script string) string {
		bin := t.TempDir()
		path := filepath.Join(bin, "gh")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
		return bin + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
	}
	oldGh := stub("if [ \"$1\" = --version ]; then echo 'gh version 2.14.7 (2022-08-25)'; exit 0; fi\nexit 1\n")
	newGh := stub("exit 0\n")
	project := func(profile string) string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".ai", "project.yaml"), profile)
		return dir
	}
	const base = "toolkit: 1\nagents: [claude-code]\n"
	cases := []struct {
		name, profile, path string
		present             bool
		status              Status
		detail              string
	}{
		{"no tracker", base, oldGh, false, OK, ""},
		{"clickup", base + "stack: { tracker: clickup }\n", oldGh, false, OK, ""},
		{"github labels", base + "stack: { tracker: { use: github, mode: labels } }\n", oldGh, false, OK, ""},
		{"github project, old gh", base + "stack: { tracker: github }\n", oldGh, true, Warn, "gh version 2.14.7 (2022-08-25) has no gh project command"},
		{"github project, new gh", base + "stack: { tracker: github }\n", newGh, true, OK, "gh has the project command"},
		{"github project, no gh", base + "stack: { tracker: github }\n", t.TempDir(), true, Warn, "gh is not on PATH"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check, ok := (Doctor{Catalog: shippedCatalog, ProjectDir: project(c.profile), PathEnv: c.path}).githubTracker()
			if ok != c.present {
				t.Fatalf("present %v, want %v", ok, c.present)
			}
			if ok {
				expectCheck(t, check, c.status, c.detail)
			}
		})
	}
}
