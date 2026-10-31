import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:path/path.dart' as p;

import '../adapters/adapter.dart';
import '../catalog/catalog.dart';
import '../guard/compile.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../package_root.dart';
import '../version.dart';
import 'guard_install.dart';
import 'project_files.dart';

/// Sync refused to run; [message] names the files or settings involved.
class SyncException implements Exception {
  SyncException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// What one sync run changed.
class SyncResult {
  const SyncResult({required this.written, required this.removed, required this.warnings});

  final List<String> written;
  final List<String> removed;
  final List<String> warnings;
}

/// Renders `.ai/project.yaml` into the agent files of one project.
/// Generated files are tracked in `.ai/lock.json`; a file edited by hand stops the run unless forced.
class Sync {
  const Sync({required this.catalog, required this.projectDir, this.compileGuard = true, this.packageRoot, this.guardHome});

  final Catalog catalog;
  final String projectDir;
  final bool compileGuard;

  /// dashcrew package root; defaults to the running dashcrew's.
  final String? packageRoot;

  /// Where the shared guard is installed; defaults to `DASHCREW_HOME`, else `~/.dashcrew`.
  final String? guardHome;

  /// Required PR sections, rendered from the catalog; `dashcrew check` reads the same headings.
  static const prTemplatePath = '.github/pull_request_template.md';

  /// The committed hook entry; it runs the guard installed for this project's guard source.
  static const guardShimPath = '.ai/bin/guard';

  static const generatedMarker = '<!-- dashcrew:generated - edit .ai/project.yaml or the project block, then run dashcrew sync -->';
  static const projectBegin = '<!-- dashcrew:project:begin -->';
  static const projectEnd = '<!-- dashcrew:project:end -->';
  static const lockPath = '.ai/lock.json';

  SyncResult run({bool force = false, bool adopt = false}) {
    final warnings = <String>[];
    final (:files, :packs, :adapters) = _render(adopt: adopt, warnings: warnings);
    final lock = _readLock();
    final lockVersion = lock['dashcrew_version'] as String?;
    if (lockVersion != null && compareVersions(lockVersion, dashcrewVersion) > 0 && !force) {
      throw SyncException('$lockPath was written by dashcrew $lockVersion, and this is $dashcrewVersion. '
          'Update dashcrew, or rerun with --force to rewrite it with $dashcrewVersion.');
    }
    final lockedFiles = (lock['files'] as Map?)?.cast<String, String>() ?? {};

    final conflicts = <String>[];
    _removeCompiledGuard();
    for (final path in {...files.keys, ...lockedFiles.keys}) {
      final file = _file(path);
      if (!file.existsSync()) continue;
      final onDisk = fileHash(path, file.readAsStringSync());
      final locked = lockedFiles[path];
      final adoptedAgentsMd = path == 'AGENTS.md' && locked == null;
      if (!adoptedAgentsMd && locked != onDisk && (locked != null || files[path] != file.readAsStringSync())) {
        conflicts.add(path);
      }
    }
    if (conflicts.isNotEmpty && !force) {
      throw SyncException('Files changed outside dashcrew (rerun with --force to overwrite):\n  ${conflicts.join('\n  ')}');
    }

    final written = <String>[];
    files.forEach((path, content) {
      if (_writeIfChanged(path, content)) written.add(path);
    });

    final removed = <String>[];
    for (final path in lockedFiles.keys) {
      if (files.containsKey(path)) continue;
      final file = _file(path);
      if (file.existsSync()) {
        file.deleteSync();
        removed.add(path);
      }
    }

    final ownedMcp = _ownedMcp(lock);
    final nowOwnedMcp = <String, List<String>>{};
    for (final adapter in adapters) {
      final merged = adapter.merge(ownedMcp[adapter.agent] ?? const {}, warnings);
      written.addAll(merged.written);
      nowOwnedMcp[adapter.agent] = merged.ownedMcp.toList()..sort();
    }
    Process.runSync('chmod', ['+x', _file(guardShimPath).path]);
    if (_dropGuardIgnore()) written.add('.gitignore');

    final guardHash = guardSourceHash(_root);
    if (compileGuard) {
      try {
        GuardInstaller(packageRoot: _root, home: guardHome).install();
      } on GuardInstallException catch (error) {
        throw SyncException(error.message);
      }
    }

    final newLock = {
      'schema': 1,
      'dashcrew_version': dashcrewVersion,
      'files': {for (final path in files.keys.toList()..sort()) path: fileHash(path, files[path]!)},
      'packs': packs,
      'mcp_servers': {for (final agent in nowOwnedMcp.keys.toList()..sort()) agent: nowOwnedMcp[agent]},
      'guard_source': guardHash,
    };
    _writeIfChanged(lockPath, '${const JsonEncoder.withIndent('  ').convert(newLock)}\n');

    return SyncResult(written: written, removed: removed, warnings: warnings);
  }

  /// Generated files as `sync` would write them now, without touching the disk.
  /// Throws [SyncException] for a missing or invalid profile, an unsupported agent, or a foreign AGENTS.md.
  /// Project packs in `.ai/packs/` are overlaid on [catalog]; a broken one throws [SyncException] too.
  Map<String, String> render({bool adopt = false, List<String>? warnings}) =>
      _render(adopt: adopt, warnings: warnings).files;

  /// MCP servers each agent's adapter wrote before; a lock that keeps a plain list predates Codex and means Claude Code.
  static Map<String, Set<String>> _ownedMcp(Map<String, Object?> lock) {
    final owned = lock['mcp_servers'];
    if (owned is List) return {'claude-code': owned.cast<String>().toSet()};
    if (owned is Map) return {for (final MapEntry(:key, :value) in owned.entries) key as String: (value as List).cast<String>().toSet()};
    return {};
  }

  /// Files plus `family/option -> builtin|project` for every rendered pack, as the lock records them.
  ({Map<String, String> files, Map<String, String> packs, List<Adapter> adapters}) _render({
    required bool adopt,
    List<String>? warnings,
  }) {
    final Catalog catalog;
    try {
      catalog = this.catalog.withProjectPacks(projectDir);
    } on CatalogException catch (error) {
      throw SyncException('Project pack: $error');
    }
    final profile = _profile(catalog);
    final adapters = [for (final agent in profile.agents) adapterFor(agent, catalog, projectDir)];

    final renderer = Renderer(catalog);
    final targets = renderer.render(profile);
    final packs = {
      for (final pack in renderer.selectedPacks(profile).toList()..sort((a, b) => a.family.compareTo(b.family)))
        '${pack.family}/${pack.option}': pack.fromProject ? 'project' : 'builtin',
    };
    final context = renderer.contextFor(profile);
    final lockedFiles = (_readLock()['files'] as Map?)?.cast<String, String>() ?? {};

    final files = <String, String>{
      'AGENTS.md': _agentsMd(targets, adopt: adopt),
      '.ai/guard.json': '${const JsonEncoder.withIndent('  ').convert(compileGuardPolicy(profile, catalog).toJson())}\n',
      guardShimPath: GuardInstaller.shim(guardSourceHash(_root)),
    };
    final prTemplate = targets.where((t) => t.target == 'pr_template').firstOrNull;
    if (prTemplate != null) {
      final existing = _file(prTemplatePath);
      if (!existing.existsSync() ||
          lockedFiles.containsKey(prTemplatePath) ||
          existing.readAsStringSync().startsWith('<!-- dashcrew:generated')) {
        files[prTemplatePath] = prTemplate.content;
      } else {
        warnings?.add('$prTemplatePath exists and was not written by dashcrew; left as is. '
            'dashcrew check still requires the headings dashcrew would write.');
      }
    }
    for (final adapter in adapters) {
      adapter.files(profile, targets, context, lockedFiles: lockedFiles, warnings: warnings).forEach((path, content) {
        final other = files[path];
        if (other != null && other != content) {
          throw StateError('$path is written by two adapters with different content (${adapter.agent})');
        }
        files[path] = content;
      });
    }
    return (files: files, packs: packs, adapters: adapters);
  }

  Profile _profile(Catalog catalog) {
    final file = _file('.ai/project.yaml');
    if (!file.existsSync()) throw SyncException('No .ai/project.yaml in $projectDir. Run dashcrew init first.');
    try {
      return Profile.parse(file.readAsStringSync(), knownOptions: catalog.knownOptions());
    } on ProfileException catch (error) {
      throw SyncException(error.toString());
    }
  }

  File _file(String relative) => ProjectFiles(projectDir).file(relative);

  Map<String, Object?> _readLock() {
    final file = _file(lockPath);
    if (!file.existsSync()) return {};
    return jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
  }

  bool _writeIfChanged(String relative, String content) => ProjectFiles(projectDir).writeIfChanged(relative, content);

  /// Hash stored in the lock for one generated file.
  /// AGENTS.md is hashed without the project block, so editing that block never counts as a hand edit.
  static String fileHash(String path, String content) {
    final hashed = path == 'AGENTS.md' ? _withoutProjectBlock(content) : content;
    return sha256.convert(utf8.encode(hashed)).toString();
  }

  static String _withoutProjectBlock(String content) {
    final begin = content.indexOf(projectBegin);
    final end = content.indexOf(projectEnd);
    if (begin < 0 || end < begin) return content;
    return content.substring(0, begin + projectBegin.length) + content.substring(end);
  }

  String _agentsMd(List<RenderedTarget> targets, {required bool adopt}) {
    final file = _file('AGENTS.md');
    var projectBlock = '';
    if (file.existsSync()) {
      final existing = file.readAsStringSync();
      final begin = existing.indexOf(projectBegin);
      final end = existing.indexOf(projectEnd);
      if (existing.contains('dashcrew:generated') && begin >= 0 && end > begin) {
        projectBlock = existing.substring(begin + projectBegin.length, end);
      } else if (adopt) {
        projectBlock = '\n${existing.trimRight()}\n';
      } else {
        throw SyncException('AGENTS.md exists and was not written by dashcrew. Rerun with --adopt to keep it as the project block.');
      }
    }
    final rules = targets.where((t) => t.target == 'agents_md').map((t) => t.content).join();
    if (projectBlock.isEmpty) projectBlock = '\n';
    return '$generatedMarker\n\n${rules.trimRight()}\n\n$projectBegin$projectBlock$projectEnd\n';
  }

  String get _root => packageRoot ?? dashcrewPackageRoot();

  /// A compiled guard from before the shim is binary, unreadable as text and never in the lock; it is dashcrew's own.
  void _removeCompiledGuard() {
    final file = _file(guardShimPath);
    if (!file.existsSync()) return;
    final head = file.openSync()..setPositionSync(0);
    final bytes = head.readSync(2);
    head.closeSync();
    if (bytes.length < 2 || bytes[0] != 0x23 || bytes[1] != 0x21) file.deleteSync();
  }

  /// Removes the `.ai/bin/` lines dashcrew once added: the shim there is meant to be committed.
  bool _dropGuardIgnore() {
    final file = _file('.gitignore');
    if (!file.existsSync()) return false;
    final current = file.readAsStringSync();
    final next = current.replaceAll('# dashcrew: compiled guard, per machine\n.ai/bin/\n', '');
    return next != current && _writeIfChanged('.gitignore', next);
  }


  /// Hash of the guard sources in the dashcrew package; the lock keeps the one the binary was built from.
  static String guardSourceHash(String packageRoot) {
    final sources = [
      p.join(packageRoot, 'bin', 'guard.dart'),
      ...Directory(p.join(packageRoot, 'lib', 'src', 'guard')).listSync().whereType<File>().map((f) => f.path).toList()..sort(),
    ];
    return sha256.convert(utf8.encode(sources.map((s) => File(s).readAsStringSync()).join('\u0000'))).toString();
  }


}
