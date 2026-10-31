/// Version of this dashcrew, kept equal to `pubspec.yaml` by a test; the lock records it.
const dashcrewVersion = '0.0.1';

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
