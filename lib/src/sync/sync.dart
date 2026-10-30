import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:path/path.dart' as p;

import '../adapters/claude_code.dart';
import '../catalog/catalog.dart';
import '../guard/compile.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';

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
  const Sync({required this.catalog, required this.projectDir, this.compileGuard = true, this.packageRoot});

  final Catalog catalog;
  final String projectDir;
  final bool compileGuard;

  /// dashcrew package root, needed only to compile the guard.
  final String? packageRoot;

  static const generatedMarker = '<!-- dashcrew:generated - edit .ai/project.yaml or the project block, then run dashcrew sync -->';
  static const projectBegin = '<!-- dashcrew:project:begin -->';
  static const projectEnd = '<!-- dashcrew:project:end -->';
  static const lockPath = '.ai/lock.json';
  static const supportedAgents = {'claude-code'};

  SyncResult run({bool force = false, bool adopt = false}) {
    final warnings = <String>[];
    final files = render(adopt: adopt, warnings: warnings);
    final lock = _readLock();
    final lockedFiles = (lock['files'] as Map?)?.cast<String, String>() ?? {};

    final conflicts = <String>[];
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

    if (_mergeSettings()) written.add('.claude/settings.json');
    final ownedMcp = ((lock['mcp_servers'] as List?) ?? const []).cast<String>().toSet();
    final mcpResult = _mergeMcp(ownedMcp, warnings);
    if (mcpResult.changed) written.add('.mcp.json');
    if (_ensureGitignore()) written.add('.gitignore');

    var guardHash = lock['guard_source'] as String?;
    if (compileGuard) guardHash = _compileGuard(guardHash);

    final newLock = {
      'schema': 1,
      'files': {for (final path in files.keys.toList()..sort()) path: fileHash(path, files[path]!)},
      'mcp_servers': mcpResult.owned.toList()..sort(),
      'guard_source': ?guardHash,
    };
    _writeIfChanged(lockPath, '${const JsonEncoder.withIndent('  ').convert(newLock)}\n');

    return SyncResult(written: written, removed: removed, warnings: warnings);
  }

  /// Generated files as `sync` would write them now, without touching the disk.
  /// Throws [SyncException] for a missing or invalid profile, an unsupported agent, or a foreign AGENTS.md.
  Map<String, String> render({bool adopt = false, List<String>? warnings}) {
    final profile = _profile();
    final unsupported = profile.agents.where((a) => !supportedAgents.contains(a)).toList();
    if (unsupported.isNotEmpty) {
      throw SyncException('Agent adapter not available yet: ${unsupported.join(', ')}. Supported: ${supportedAgents.join(', ')}.');
    }

    final renderer = Renderer(catalog);
    final targets = renderer.render(profile);
    final context = renderer.contextFor(profile);
    final lockedFiles = (_readLock()['files'] as Map?)?.cast<String, String>() ?? {};

    final files = <String, String>{
      'AGENTS.md': _agentsMd(targets, adopt: adopt),
      '.ai/guard.json': '${const JsonEncoder.withIndent('  ').convert(compileGuardPolicy(profile, catalog).toJson())}\n',
      ...ClaudeCodeAdapter(catalog).files(profile, targets, context),
    };
    final claudeMd = _file('CLAUDE.md');
    if (!claudeMd.existsSync() || lockedFiles.containsKey('CLAUDE.md')) {
      files['CLAUDE.md'] = '@AGENTS.md\n';
    } else if (!claudeMd.readAsStringSync().contains('@AGENTS.md')) {
      warnings?.add('CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md" so Claude Code loads the rules.');
    }
    return files;
  }

  Profile _profile() {
    final file = _file('.ai/project.yaml');
    if (!file.existsSync()) throw SyncException('No .ai/project.yaml in $projectDir. Run dashcrew init first.');
    try {
      return Profile.parse(file.readAsStringSync(), knownOptions: catalog.knownOptions());
    } on ProfileException catch (error) {
      throw SyncException(error.toString());
    }
  }

  File _file(String relative) => File(p.join(projectDir, relative));

  Map<String, Object?> _readLock() {
    final file = _file(lockPath);
    if (!file.existsSync()) return {};
    return jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
  }

  bool _writeIfChanged(String relative, String content) {
    final file = _file(relative);
    if (file.existsSync() && file.readAsStringSync() == content) return false;
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
    return true;
  }

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

  bool _mergeSettings() {
    final file = _file('.claude/settings.json');
    final settings = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
    final hooks = (settings['hooks'] as Map?)?.cast<String, Object?>() ?? <String, Object?>{};
    final preToolUse = [...(hooks['PreToolUse'] as List?) ?? const []];

    final ours = ClaudeCodeAdapter.guardHook;
    final index = preToolUse.indexWhere((entry) => jsonEncode(entry).contains('.ai/bin/guard'));
    if (index >= 0) {
      preToolUse[index] = ours;
    } else {
      preToolUse.add(ours);
    }
    hooks['PreToolUse'] = preToolUse;
    settings['hooks'] = hooks;
    return _writeIfChanged('.claude/settings.json', '${const JsonEncoder.withIndent('  ').convert(settings)}\n');
  }

  ({bool changed, Set<String> owned}) _mergeMcp(Set<String> owned, List<String> warnings) {
    final file = _file('.mcp.json');
    final config = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
    final servers = (config['mcpServers'] as Map?)?.cast<String, Object?>() ?? <String, Object?>{};
    final nowOwned = {...owned};

    ClaudeCodeAdapter.mcpServers.forEach((name, server) {
      final existing = servers[name];
      if (existing == null || owned.contains(name)) {
        servers[name] = server;
        nowOwned.add(name);
      } else if (jsonEncode(existing) != jsonEncode(server)) {
        warnings.add('.mcp.json already defines "$name" differently; left as is.');
      }
    });
    config['mcpServers'] = servers;
    final changed = _writeIfChanged('.mcp.json', '${const JsonEncoder.withIndent('  ').convert(config)}\n');
    return (changed: changed, owned: nowOwned);
  }

  bool _ensureGitignore() {
    final file = _file('.gitignore');
    final current = file.existsSync() ? file.readAsStringSync() : '';
    if (current.split('\n').any((line) => line.trim() == '.ai/bin/')) return false;
    final prefix = current.isEmpty || current.endsWith('\n') ? current : '$current\n';
    return _writeIfChanged('.gitignore', '$prefix# dashcrew: compiled guard, per machine\n.ai/bin/\n');
  }

  /// Hash of the guard sources in the dashcrew package; the lock keeps the one the binary was built from.
  static String guardSourceHash(String packageRoot) {
    final sources = [
      p.join(packageRoot, 'bin', 'guard.dart'),
      ...Directory(p.join(packageRoot, 'lib', 'src', 'guard')).listSync().whereType<File>().map((f) => f.path).toList()..sort(),
    ];
    return sha256.convert(utf8.encode(sources.map((s) => File(s).readAsStringSync()).join('\u0000'))).toString();
  }

  /// Recompiles `.ai/bin/guard` when the guard sources changed or the binary is missing.
  String _compileGuard(String? previousHash) {
    final root = packageRoot;
    if (root == null) throw SyncException('Cannot compile the guard: dashcrew package root unknown.');
    final hash = guardSourceHash(root);
    final binary = _file('.ai/bin/guard');
    if (hash == previousHash && binary.existsSync()) return hash;

    binary.parent.createSync(recursive: true);
    final result = Process.runSync(
        Platform.resolvedExecutable, ['compile', 'exe', p.join(root, 'bin', 'guard.dart'), '-o', binary.path],
        workingDirectory: root);
    if (result.exitCode != 0) throw SyncException('Guard compile failed:\n${result.stderr}');
    return hash;
  }
}
