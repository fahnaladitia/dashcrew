/// Version of this dashcrew, kept equal to `pubspec.yaml` by a test; the lock records it.
/// It is older than [guardMinimumVersion], so the guard shim refuses a Dart-installed dashcrew.
const dashcrewVersion = '0.0.1';

/// Oldest dashcrew whose `dashcrew guard` the shim accepts: the first release of the Go engine.
/// Sync writes it into `.ai/bin/guard` and the lock's `guard_min_version`.
const guardMinimumVersion = '0.1.0';

/// Orders two semantic versions by major, minor and patch; build metadata is ignored,
/// and a pre-release sorts before its release.
int compareVersions(String a, String b) {
  (List<int>, bool) parse(String version) {
    final core = version.split('+').first;
    final dash = core.indexOf('-');
    final numbers = (dash < 0 ? core : core.substring(0, dash)).split('.').map((part) => int.tryParse(part) ?? 0).toList();
    while (numbers.length < 3) {
      numbers.add(0);
    }
    return (numbers, dash >= 0);
  }

  final (left, leftPre) = parse(a);
  final (right, rightPre) = parse(b);
  for (var i = 0; i < 3; i++) {
    if (left[i] != right[i]) return left[i].compareTo(right[i]);
  }
  if (leftPre == rightPre) return 0;
  return leftPre ? -1 : 1;
}

/// True for `major.minor.patch` with an optional `-pre-release` and `+build`, the shape the guard shim reads.
bool isSemanticVersion(String version) {
  final core = version.split('+').first.split('-').first;
  final parts = core.split('.');
  return parts.length == 3 && parts.every((part) => RegExp(r'^[0-9]+$').hasMatch(part));
}
