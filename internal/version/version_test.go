package version

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestVersionMatchesPubspec(t *testing.T) {
	pubspec, err := os.ReadFile(filepath.Join("..", "..", "pubspec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^version: (\S+)$`).FindStringSubmatch(string(pubspec))
	if match == nil || match[1] != Version {
		t.Errorf("pubspec %v, Version %s", match, Version)
	}
}

func TestCompareOrdersByMajorMinorPatchAndIgnoresBuildMetadata(t *testing.T) {
	cases := []struct {
		a, b string
		sign int
	}{
		{"0.1.0", "0.0.9", 1},
		{"1.2.3", "1.10.0", -1},
		{"1.2.3+4", "1.2.3", 0},
		{"2.0.0-dev.1", "2.0.0", -1},
		{"2.0.0", "2.0.0-dev.1", 1},
		{"1", "1.0.0", 0},
		{"x.1", "0.1", 0},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if (got > 0) != (c.sign > 0) || (got < 0) != (c.sign < 0) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
}
