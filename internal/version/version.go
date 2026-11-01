// Package version is the dashcrew version the lock records, the minimum version the guard shim accepts, and
// semantic version ordering.
package version

import (
	"runtime/debug"
	"strconv"
	"strings"
)

// Version is stamped by release builds:
// `-ldflags "-X github.com/fahnaladitia/dashcrew/internal/version.Version=0.1.0"`. Empty when not stamped.
var Version = ""

// Unstamped is the version of a build that carries none, such as a test or a `go build -buildvcs=false`.
// The goldens record it, and an old Dart-installed dashcrew reports it too, so the shim refuses that one.
const Unstamped = "0.0.1"

// GuardMinimum is the oldest dashcrew whose `dashcrew guard` the shim accepts: the first release with the Go guard.
// Raise it when `.ai/guard.json` needs a newer guard; sync writes it into the shim and the lock.
const GuardMinimum = "0.1.0"

// Current is the stamped Version, else the module version of a `go install <module>@<version>` build, else
// Unstamped. A leading `v` is dropped.
func Current() string {
	if Version != "" {
		return strings.TrimPrefix(Version, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return fromModule(info.Main.Version)
	}
	return Unstamped
}

// fromModule is a module version without its `v`; Unstamped for `(devel)`, an empty one or any other shape.
func fromModule(module string) string {
	if trimmed, ok := strings.CutPrefix(module, "v"); ok && IsSemantic(trimmed) {
		return trimmed
	}
	return Unstamped
}

// IsSemantic is true for `major.minor.patch` with an optional `-pre-release` and `+build`, the shape the shim reads.
func IsSemantic(version string) bool {
	core, _, _ := strings.Cut(version, "+")
	core, _, _ = strings.Cut(core, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return false
		}
	}
	return true
}

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
