import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

import '../yaml_util.dart';

/// A worktree command refused to run; [message] says why and what the user can do.
class WorktreeException implements Exception {
  WorktreeException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// One task worktree as `list` reports it; [upstream] is null for a branch never pushed.
class WorktreeInfo {
  const WorktreeInfo({
    required this.name,
    required this.path,
    required this.branch,
    required this.base,
    required this.upstream,
    required this.dirty,
    required this.missingLinks,
  });

  final String name;
  final String path;
  final String branch;
  final String? base;
  final String? upstream;
  final int dirty;
  final List<String> missingLinks;
}

/// Task worktrees of one repository, kept in `<main checkout>.worktrees/<name>`.
/// The only writing git commands are `worktree add --no-track -b`, `branch --unset-upstream`,
/// `config branch.<b>.dashcrew-base` and `worktree remove`; nothing commits, pushes or deletes a branch.
class Worktrees {
  Worktrees({required String projectDir}) : gitDir = _commonGitDir(projectDir);

  /// The repository's shared git directory, also when [Worktrees] is created from inside a task worktree.
  final String gitDir;

  /// The main checkout that holds [gitDir].
  String get mainDir => p.dirname(gitDir);

  String get worktreesDir => '$mainDir.worktrees';

  static const _toolkitEntries = ['AGENTS.md', 'CLAUDE.md', '.claude', '.mcp.json', '.ai'];

  static String _commonGitDir(String projectDir) {
    final result = Process.runSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], workingDirectory: projectDir);
    if (result.exitCode != 0) throw WorktreeException('$projectDir is not inside a git repository.');
    return (result.stdout as String).trim();
  }

  /// Creates `<branch>` from [base] in a new worktree; [base] must exist and is recorded without `origin/`.
  WorktreeInfo create(String branch, String base, {String? name, bool pubGet = true, required void Function(String) out}) {
    final folder = name ?? branch.replaceAll('/', '-');
    final path = p.join(worktreesDir, folder);
    if (_git(mainDir, ['check-ref-format', '--branch', branch]).exitCode != 0) {
      throw WorktreeException('"$branch" is not a valid branch name.');
    }
    if (_git(mainDir, ['rev-parse', '--verify', '--quiet', '$base^{commit}']).exitCode != 0) {
      throw WorktreeException('Base $base not found. Fetch it or name an existing branch.');
    }
    if (_git(mainDir, ['rev-parse', '--verify', '--quiet', 'refs/heads/$branch']).exitCode == 0) {
      throw WorktreeException('Branch $branch already exists.');
    }
    if (FileSystemEntity.typeSync(path, followLinks: false) != FileSystemEntityType.notFound) {
      throw WorktreeException('Folder $path already exists.');
    }

    _gitOrThrow(mainDir, ['worktree', 'add', '--quiet', '--no-track', '-b', branch, path, base]);
    _dropUpstream(path, out);
    final recordedBase = base.startsWith('origin/') ? base.substring('origin/'.length) : base;
    _gitOrThrow(mainDir, ['config', 'branch.$branch.dashcrew-base', recordedBase]);
    final linked = _link(path, dryRun: false);

    out('path: $path');
    out('branch: $branch (base $recordedBase)');
    out('upstream: none');
    out('status: ${(_git(path, ['status', '-sb']).stdout as String).split('\n').first}');
    out('links: ${linked.isEmpty ? 'none' : linked.join(', ')}');
    out('pub get: ${pubGet ? _pubGet(path) : 'skipped'}');
    return list().singleWhere((info) => info.path == path);
  }

  /// Adds missing toolkit links to worktree [name] and drops an upstream it should not have; returns the added links.
  List<String> link(String name, {required void Function(String) out}) {
    final path = _existing(name);
    _dropUpstream(path, out);
    final linked = _link(path, dryRun: false);
    out(linked.isEmpty ? 'links: up to date' : 'linked: ${linked.join(', ')}');
    return linked;
  }

  /// Every task worktree, main checkout excluded.
  List<WorktreeInfo> list() {
    final porcelain = _gitOrThrow(mainDir, ['worktree', 'list', '--porcelain']);
    final infos = <WorktreeInfo>[];
    for (final block in porcelain.split('\n\n')) {
      final lines = block.split('\n');
      final path = lines.firstWhere((l) => l.startsWith('worktree '), orElse: () => '').replaceFirst('worktree ', '');
      final branchLine = lines.firstWhere((l) => l.startsWith('branch '), orElse: () => '');
      if (path.isEmpty || path == mainDir || branchLine.isEmpty || !Directory(path).existsSync()) continue;
      final branch = branchLine.replaceFirst('branch refs/heads/', '');
      final upstream = _git(path, ['rev-parse', '--abbrev-ref', '@{u}']);
      final status = (_git(path, ['status', '--porcelain']).stdout as String).trim();
      infos.add(WorktreeInfo(
        name: p.basename(path),
        path: path,
        branch: branch,
        base: baseOf(branch),
        upstream: upstream.exitCode == 0 ? (upstream.stdout as String).trim() : null,
        dirty: status.isEmpty ? 0 : status.split('\n').length,
        missingLinks: _link(path, dryRun: true),
      ));
    }
    return infos;
  }

  /// Removes a clean worktree [name]; the branch and the linked toolkit in the main checkout stay.
  void remove(String name, {required void Function(String) out}) {
    final path = _existing(name);
    final status = (_git(path, ['status', '--porcelain']).stdout as String).trim();
    if (status.isNotEmpty) {
      throw WorktreeException('$name has uncommitted changes (${status.split('\n').length} lines). Commit or discard them first.');
    }
    _gitOrThrow(mainDir, ['worktree', 'remove', path]);
    out('removed: $path (branch kept)');
  }

  /// Base recorded by [create], without `origin/`.
  String? baseOf(String branch) {
    final result = _git(mainDir, ['config', '--get', 'branch.$branch.dashcrew-base']);
    return result.exitCode == 0 ? (result.stdout as String).trim() : null;
  }

  String _existing(String name) {
    final path = p.join(worktreesDir, name);
    if (!list().any((info) => info.path == path)) throw WorktreeException('No worktree named $name in $worktreesDir.');
    return path;
  }

  void _dropUpstream(String path, void Function(String) out) {
    if (_git(path, ['rev-parse', '--abbrev-ref', '@{u}']).exitCode != 0) return;
    _gitOrThrow(path, ['branch', '--unset-upstream']);
    out('upstream removed: a new branch tracks nothing until its first push');
  }

  /// Links toolkit entries the main checkout ignores and the worktree lacks; recurses into other folders.
  /// An entry the main checkout does not ignore is never linked: the shared exclude file would hide it there too.
  List<String> _link(String worktree, {required bool dryRun}) {
    final linked = <String>[];
    void visit(String relative) {
      final source = p.join(mainDir, relative);
      if (FileSystemEntity.typeSync(source, followLinks: false) == FileSystemEntityType.notFound) return;
      final target = p.join(worktree, relative);
      final type = FileSystemEntity.typeSync(target, followLinks: false);
      final sourceIsDir = FileSystemEntity.isDirectorySync(source);
      if (type == FileSystemEntityType.notFound && _git(mainDir, ['check-ignore', '-q', relative]).exitCode == 0) {
        if (!dryRun) Link(target).createSync(source, recursive: true);
        linked.add(relative);
      } else if ((type == FileSystemEntityType.notFound || type == FileSystemEntityType.directory) && sourceIsDir) {
        final children = Directory(source).listSync(followLinks: false).map((e) => p.basename(e.path)).toList()..sort();
        for (final child in children) {
          visit(p.posix.join(relative, child));
        }
      }
    }

    final entries = {..._toolkitEntries, ..._profilePaths()};
    for (final entry in entries) {
      if (!entries.any((other) => other != entry && p.isWithin(other, entry))) visit(entry);
    }
    if (!dryRun && linked.isNotEmpty) _exclude(worktree, linked);
    return linked;
  }

  /// `paths.plans`, `paths.pr` and `paths.memory` from the main checkout's profile, with their defaults.
  List<String> _profilePaths() {
    final defaults = {'plans': '.ai/plans', 'pr': '.ai/pr', 'memory': '.ai/memory'};
    final file = File(p.join(mainDir, '.ai', 'project.yaml'));
    final yaml = file.existsSync() ? plainYaml(loadYaml(file.readAsStringSync())) : null;
    final paths = yaml is Map<String, Object?> && yaml['paths'] is Map ? yaml['paths'] as Map : const {};
    return [
      for (final key in defaults.keys) p.posix.normalize((paths[key] as String?) ?? defaults[key]!),
    ];
  }

  void _exclude(String worktree, List<String> linked) {
    final excludePath = (_git(worktree, ['rev-parse', '--path-format=absolute', '--git-path', 'info/exclude']).stdout as String).trim();
    final file = File(excludePath);
    final current = file.existsSync() ? file.readAsStringSync() : '';
    final lines = current.split('\n').toSet();
    final missing = [for (final path in linked) if (!lines.contains('/$path')) '/$path'];
    if (missing.isEmpty) return;
    final prefix = current.isEmpty || current.endsWith('\n') ? current : '$current\n';
    file
      ..parent.createSync(recursive: true)
      ..writeAsStringSync('$prefix${missing.join('\n')}\n');
  }

  String _pubGet(String path) {
    if (!File(p.join(path, 'pubspec.yaml')).existsSync()) return 'skipped (no pubspec.yaml)';
    final fvm = File(p.join(path, '.fvmrc')).existsSync();
    final command = fvm ? ['fvm', 'flutter', 'pub', 'get'] : ['flutter', 'pub', 'get'];
    final result = Process.runSync(command.first, command.sublist(1), workingDirectory: path, runInShell: true);
    if (result.exitCode == 0) return 'ok (${command.join(' ')})';
    return 'failed (exit ${result.exitCode}); rerun: cd $path && ${command.join(' ')}';
  }

  static ProcessResult _git(String dir, List<String> args) => Process.runSync('git', args, workingDirectory: dir);

  static String _gitOrThrow(String dir, List<String> args) {
    final result = _git(dir, args);
    if (result.exitCode != 0) throw WorktreeException('git ${args.join(' ')} failed: ${(result.stderr as String).trim()}');
    return result.stdout as String;
  }
}
