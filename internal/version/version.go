// Package version is the dashcrew version the lock records, and semantic version ordering.
package version

import (
	"strconv"
	"strings"
)

// Version of this dashcrew, kept equal to `pubspec.yaml` by a test until the Dart engine is removed.
const Version = "0.0.1"

// Compare orders two semantic versions by major, minor and patch: negative, zero or positive.
// Build metadata is ignored, a pre-release sorts before its release, and a part that is not a number counts as 0.
func Compare(a, b string) int {
	left, leftPre := parse(a)
	right, rightPre := parse(b)
	for i := 0; i < 3; i++ {
		if left[i] != right[i] {
			if left[i] < right[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case leftPre == rightPre:
		return 0
	case leftPre:
		return -1
	default:
		return 1
	}
}

func parse(version string) ([3]int, bool) {
	core, _, _ := strings.Cut(version, "+")
	dash := strings.Index(core, "-")
	if dash >= 0 {
		core = core[:dash]
	}
	var numbers [3]int
	for i, part := range strings.Split(core, ".") {
		if i >= 3 {
			break
		}
		if n, err := strconv.Atoi(part); err == nil {
			numbers[i] = n
		}
	}
	return numbers, dash >= 0
}
