package dashcrew

import "embed"

// DartGuardSourceFS holds the Dart guard sources at their repository paths. The lock's `guard_source` and the
// shim are keyed by their hash until stage G7 replaces that hash with the dashcrew version.
//
//go:embed bin/guard.dart lib/src/guard
var DartGuardSourceFS embed.FS
