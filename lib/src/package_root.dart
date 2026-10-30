import 'dart:isolate';

import 'package:path/path.dart' as p;

/// Root of the installed dashcrew package (holds `core/`, `packs/`, `bin/guard.dart`).
/// Works from `dart run`, a path activation and a pub-cache activation alike.
String dashcrewPackageRoot() {
  final library = Isolate.resolvePackageUriSync(Uri.parse('package:dashcrew/dashcrew.dart'));
  if (library == null) throw StateError('package:dashcrew cannot be resolved');
  return p.dirname(p.dirname(library.toFilePath()));
}
