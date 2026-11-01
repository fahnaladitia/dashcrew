package conformance

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// platformTerms are Flutter and Dart terms that must not reach a project on another platform.
var platformTerms = regexp.MustCompile(`(?i)flutter|dart|pubspec|widget|semantics|buildcontext`)

func TestSampleOtherRendersNoFlutterOrDartTermInAnyFile(t *testing.T) {
	expected := tree(t, filepath.Join(goldenDir(t), "sample-other", "expected"))
	if len(expected) == 0 {
		t.Fatal("sample-other/expected is empty")
	}
	var found []string
	for _, path := range sortedKeys(expected) {
		for i, line := range strings.Split(expected[path], "\n") {
			for _, match := range platformTerms.FindAllString(line, -1) {
				found = append(found, fmt.Sprintf("%s:%d: %s", path, i+1, match))
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("platform terms in sample-other:\n%s", strings.Join(found, "\n"))
	}
}
