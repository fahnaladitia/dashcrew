import 'dart:io';

import 'package:path/path.dart' as p;

import 'sync.dart';

/// The guard could not be installed; [message] names the reason and the fix.
class GuardInstallException implements Exception {
  GuardInstallException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// One compiled guard per machine and guard source, at `<home>/guard/<hash>/guard`, shared by every project.
/// The folder is named after the guard source hash, not the dashcrew version: unreleased commits share a version.
class GuardInstaller {
  GuardInstaller({required this.packageRoot, String? home}) : home = home ?? defaultHome(Platform.environment) ?? '';

  final String packageRoot;
  final String home;

  /// `DASHCREW_HOME`, else `~/.dashcrew`; null when no home directory is known.
  static String? defaultHome(Map<String, String> environment) {
    final override = environment['DASHCREW_HOME'];
    if (override != null && override.isNotEmpty) return override;
    final user = environment['HOME'] ?? environment['USERPROFILE'];
    return user == null ? null : p.join(user, '.dashcrew');
  }

  /// Hash of this dashcrew's guard sources, the same value the lock keeps as `guard_source`.
  String get sourceHash => Sync.guardSourceHash(packageRoot);

  /// Where the guard for [hash] (default: this dashcrew's) lives.
  String binaryPath([String? hash]) => p.join(home, 'guard', (hash ?? sourceHash).substring(0, 16), 'guard');

  /// Compiles the guard unless it is already installed; [expect] refuses a dashcrew whose guard source differs,
  /// so a teammate on another dashcrew commit never fills a project's slot with a different guard.
  ({String path, bool compiled}) install({String? expect, bool force = false}) {
    if (home.isEmpty) throw GuardInstallException('No home directory; set DASHCREW_HOME.');
    final hash = sourceHash;
    if (expect != null && expect != hash) {
      throw GuardInstallException('This project expects guard ${expect.substring(0, 16)}, but this dashcrew builds '
          '${hash.substring(0, 16)}. Install the dashcrew ref the team uses, then run dashcrew sync.');
    }
    final target = File(binaryPath(hash));
    if (target.existsSync() && !force) return (path: target.path, compiled: false);

    target.parent.createSync(recursive: true);
    final temporary = '${target.path}.tmp-$pid';
    final result = Process.runSync(
      Platform.resolvedExecutable,
      ['compile', 'exe', p.join(packageRoot, 'bin', 'guard.dart'), '-o', temporary],
      workingDirectory: packageRoot,
    );
    if (result.exitCode != 0) {
      if (File(temporary).existsSync()) File(temporary).deleteSync();
      throw GuardInstallException('Guard compile failed:\n${result.stderr}');
    }
    File(temporary).renameSync(target.path);
    return (path: target.path, compiled: true);
  }
}
