package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/doctor"
)

func TestAbsoluteCleansThePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{".": cwd, "a/../b": filepath.Join(cwd, "b"), "./x/": filepath.Join(cwd, "x")} {
		if got, err := absolute(path); err != nil || got != want {
			t.Errorf("absolute(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
}

// TestPrintChecksKeepsLongIDsApartFromTheirDetail pins a gap of two spaces after the longest ID; a report whose IDs
// are all shorter than 15 characters keeps the 16-column layout.
func TestPrintChecksKeepsLongIDsApartFromTheirDetail(t *testing.T) {
	var out bytes.Buffer
	printChecks(&out, []doctor.Check{
		{ID: "hook", Status: doctor.OK, Detail: "a"},
		{ID: "codex-skills-budget", Status: doctor.OK, Detail: "b"},
	}, false)
	if want := "ok    hook                 a\nok    codex-skills-budget  b\n"; out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}

	out.Reset()
	printChecks(&out, []doctor.Check{{ID: "hook", Status: doctor.OK, Detail: "a"}, {ID: "guard-binary", Status: doctor.Warn, Detail: "b"}}, false)
	if want := "ok    hook            a\nwarn  guard-binary    b\n"; out.String() != want {
		t.Errorf("short IDs: got\n%s\nwant\n%s", out.String(), want)
	}
	if strings.Contains(out.String(), "\t") {
		t.Error("tab in the table")
	}
}
