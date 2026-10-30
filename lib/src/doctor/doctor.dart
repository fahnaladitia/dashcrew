import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import '../adapters/claude_code.dart';
import '../catalog/catalog.dart';
import '../guard/guard.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/sync.dart';

enum CheckStatus { ok, warn, fail, skip }

/// Result of one doctor check; [detail] is a single line meant for the terminal.
class DoctorCheck {
  const DoctorCheck(this.id, this.status, this.detail);

  final String id;
  final CheckStatus status;
  final String detail;
}

/// Read-only health check of the toolkit `sync` wrote into one project.
/// Never writes; `dashcrew sync` is the repair path for everything it reports.
class Doctor {
  const Doctor({required this.catalog, required this.projectDir, this.packageRoot, this.userSkillsDir});

  final Catalog catalog;
  final String projectDir;

  /// dashcrew package root; without it the guard staleness check is skipped.
  final String? packageRoot;

  /// Where user-level skills live; defaults to `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`.
  final String? userSkillsDir;

  static const agentsMdLimit = 32 * 1024;
  static final _skillReference = RegExp(r'`([a-z0-9][a-z0-9-]*)` skill|skill `([a-z0-9][a-z0-9-]*)`');

  Future<List<DoctorCheck>> run() async {
    final profile = _profileCheck();
    final (lockCheck, lock) = _lockCheck();
    final lockedFiles = (lock?['files'] as Map?)?.cast<String, String>();
    return [
      profile,
      lockCheck,
      _drift(lockedFiles),
      _stale(lockedFiles, profileOk: profile.status == CheckStatus.ok),
      _guardPolicy(),
      await _guardBinary(lock),
      _hook(),
      _claudeMd(),
      _mcp(),
      _skills(lockedFiles),
      _agentsMdSize(),
      _gitignore(),
    ];
  }

  File _file(String relative) => File(p.join(projectDir, relative));

  DoctorCheck _profileCheck() {
    const id = 'profile';
    final file = _file('.ai/project.yaml');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '.ai/project.yaml missing; run dashcrew init');
    try {
      Profile.parse(file.readAsStringSync(), knownOptions: catalog.knownOptions());
      return const DoctorCheck(id, CheckStatus.ok, '.ai/project.yaml valid');
    } catch (error) {
      return DoctorCheck(id, CheckStatus.fail, '.ai/project.yaml invalid: $error');
    }
  }

  (DoctorCheck, Map<String, Object?>?) _lockCheck() {
    const id = 'lock';
    final file = _file(Sync.lockPath);
    if (!file.existsSync()) return (const DoctorCheck(id, CheckStatus.fail, '${Sync.lockPath} missing; run dashcrew sync'), null);
    final Object? lock;
    try {
      lock = jsonDecode(file.readAsStringSync());
    } on FormatException catch (error) {
      return (DoctorCheck(id, CheckStatus.fail, '${Sync.lockPath} is not JSON: ${error.message}'), null);
    }
    if (lock is! Map<String, Object?> || lock['schema'] != 1) {
      return (const DoctorCheck(id, CheckStatus.fail, '${Sync.lockPath} has an unknown schema; run dashcrew sync --force'), null);
    }
    return (const DoctorCheck(id, CheckStatus.ok, '${Sync.lockPath} schema 1'), lock);
  }

  DoctorCheck _drift(Map<String, String>? lockedFiles) {
    const id = 'drift';
    if (lockedFiles == null) return const DoctorCheck(id, CheckStatus.skip, 'needs the lock');
    final missing = <String>[];
    final edited = <String>[];
    for (final path in lockedFiles.keys.toList()..sort()) {
      final file = _file(path);
      if (!file.existsSync()) {
        missing.add(path);
      } else if (Sync.fileHash(path, file.readAsStringSync()) != lockedFiles[path]) {
        edited.add(path);
      }
    }
    if (missing.isEmpty && edited.isEmpty) {
      return DoctorCheck(id, CheckStatus.ok, '${lockedFiles.length} generated files match the lock');
    }
    final parts = [
      if (missing.isNotEmpty) 'missing: ${missing.join(', ')}',
      if (edited.isNotEmpty) 'edited by hand: ${edited.join(', ')}',
    ];
    return DoctorCheck(id, CheckStatus.fail, '${parts.join('; ')} (dashcrew sync --force restores them)');
  }

  DoctorCheck _stale(Map<String, String>? lockedFiles, {required bool profileOk}) {
    const id = 'stale';
    if (lockedFiles == null || !profileOk) return const DoctorCheck(id, CheckStatus.skip, 'needs a valid profile and the lock');
    final Map<String, String> expected;
    try {
      expected = Sync(catalog: catalog, projectDir: projectDir, compileGuard: false).render();
    } on SyncException catch (error) {
      return DoctorCheck(id, CheckStatus.fail, error.message);
    } on RenderException catch (error) {
      return DoctorCheck(id, CheckStatus.fail, error.toString());
    }
    final changed = [
      for (final path in {...expected.keys, ...lockedFiles.keys}.toList()..sort())
        if (expected[path] == null || Sync.fileHash(path, expected[path]!) != lockedFiles[path]) path,
    ];
    if (changed.isEmpty) return const DoctorCheck(id, CheckStatus.ok, 'output matches the current profile and catalog');
    final shown = changed.length > 5 ? '${changed.take(5).join(', ')}, ...' : changed.join(', ');
    return DoctorCheck(id, CheckStatus.warn, '${changed.length} files would change ($shown); run dashcrew sync');
  }

  DoctorCheck _guardPolicy() {
    const id = 'guard-policy';
    final file = _file('.ai/guard.json');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '.ai/guard.json missing; run dashcrew sync');
    try {
      GuardPolicy.fromJson(jsonDecode(file.readAsStringSync()) as Map<String, Object?>);
      return const DoctorCheck(id, CheckStatus.ok, '.ai/guard.json valid');
    } catch (error) {
      return DoctorCheck(id, CheckStatus.fail, '.ai/guard.json invalid: $error');
    }
  }

  /// Runs the binary on a `git push` event, which every policy blocks; the guard fails open, so exit 0 means broken.
  Future<DoctorCheck> _guardBinary(Map<String, Object?>? lock) async {
    const id = 'guard-binary';
    final binary = _file('.ai/bin/guard');
    if (!binary.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '.ai/bin/guard missing; run dashcrew sync');

    final int exitCode;
    try {
      final process = await Process.start(binary.path, [_file('.ai/guard.json').path], workingDirectory: projectDir);
      process.stdout.drain<void>();
      process.stderr.drain<void>();
      process.stdin.write(jsonEncode({
        'tool_name': 'Bash',
        'tool_input': {'command': 'git push'},
        'cwd': projectDir,
      }));
      await process.stdin.close();
      exitCode = await process.exitCode.timeout(const Duration(seconds: 10), onTimeout: () {
        process.kill();
        return -1;
      });
    } on ProcessException catch (error) {
      return DoctorCheck(id, CheckStatus.fail, '.ai/bin/guard cannot run: ${error.message}; run dashcrew sync');
    }
    if (exitCode != 2) {
      return DoctorCheck(id, CheckStatus.fail, '.ai/bin/guard did not block git push (exit $exitCode); check .ai/guard.json');
    }

    final root = packageRoot;
    if (root != null && lock?['guard_source'] != Sync.guardSourceHash(root)) {
      return const DoctorCheck(id, CheckStatus.warn, 'built from older guard sources; run dashcrew sync');
    }
    return const DoctorCheck(id, CheckStatus.ok, '.ai/bin/guard blocks git push');
  }

  DoctorCheck _hook() {
    const id = 'hook';
    final file = _file('.claude/settings.json');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '.claude/settings.json missing; run dashcrew sync');
    final List<Object?> preToolUse;
    try {
      final settings = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
      preToolUse = ((settings['hooks'] as Map?)?['PreToolUse'] as List?) ?? const [];
    } catch (error) {
      return DoctorCheck(id, CheckStatus.fail, '.claude/settings.json invalid: $error');
    }
    final ours = jsonEncode(ClaudeCodeAdapter.guardHook);
    if (preToolUse.any((entry) => jsonEncode(entry) == ours)) {
      return const DoctorCheck(id, CheckStatus.ok, 'PreToolUse guard hook present');
    }
    final edited = preToolUse.any((entry) => jsonEncode(entry).contains('.ai/bin/guard'));
    return DoctorCheck(id, CheckStatus.fail,
        '${edited ? 'PreToolUse guard hook differs from what sync writes' : 'PreToolUse guard hook missing'}; run dashcrew sync');
  }

  DoctorCheck _claudeMd() {
    const id = 'claude-md';
    final file = _file('CLAUDE.md');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.fail, 'CLAUDE.md missing; run dashcrew sync');
    if (!file.readAsStringSync().contains('@AGENTS.md')) {
      return const DoctorCheck(id, CheckStatus.fail, 'CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md"');
    }
    return const DoctorCheck(id, CheckStatus.ok, 'CLAUDE.md imports AGENTS.md');
  }

  DoctorCheck _mcp() {
    const id = 'mcp';
    final file = _file('.mcp.json');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.warn, '.mcp.json missing; run dashcrew sync');
    final Map<Object?, Object?> servers;
    try {
      servers = ((jsonDecode(file.readAsStringSync()) as Map)['mcpServers'] as Map?) ?? const {};
    } catch (error) {
      return DoctorCheck(id, CheckStatus.warn, '.mcp.json invalid: $error');
    }
    final problems = [
      for (final MapEntry(key: name, value: server) in ClaudeCodeAdapter.mcpServers.entries)
        if (servers[name] == null)
          '$name missing'
        else if (jsonEncode(servers[name]) != jsonEncode(server))
          '$name defined differently',
    ];
    if (problems.isEmpty) return DoctorCheck(id, CheckStatus.ok, 'defined: ${ClaudeCodeAdapter.mcpServers.keys.join(', ')}');
    return DoctorCheck(id, CheckStatus.warn, problems.join(', '));
  }

  /// A missing catalog skill fails; a missing external one only warns, since plugins install skills elsewhere.
  DoctorCheck _skills(Map<String, String>? lockedFiles) {
    const id = 'skills';
    if (lockedFiles == null) return const DoctorCheck(id, CheckStatus.skip, 'needs the lock');
    final referenced = <String>{};
    for (final path in lockedFiles.keys.where((path) => path.endsWith('.md'))) {
      final file = _file(path);
      if (!file.existsSync()) continue;
      for (final match in _skillReference.allMatches(file.readAsStringSync())) {
        referenced.add(match.group(1) ?? match.group(2)!);
      }
    }

    final dashcrewSkills = {
      ...catalog.skills.keys,
      ...catalog.workflows.keys,
      for (final options in catalog.packs.values)
        for (final pack in options.values) ...pack.skills.keys,
    };
    final userDir = userSkillsDir ?? defaultUserSkillsDir();
    final missing = <String>[];
    final external = <String>[];
    for (final name in referenced.toList()..sort()) {
      if (_file(p.join('.claude', 'skills', name, 'SKILL.md')).existsSync()) continue;
      if (dashcrewSkills.contains(name)) {
        missing.add(name);
      } else if (userDir == null || !File(p.join(userDir, name, 'SKILL.md')).existsSync()) {
        external.add(name);
      }
    }
    if (missing.isNotEmpty) {
      return DoctorCheck(id, CheckStatus.fail, 'dashcrew skills missing: ${missing.join(', ')}; run dashcrew sync');
    }
    if (external.isNotEmpty) {
      return DoctorCheck(id, CheckStatus.warn, 'not found locally (install, or ignore if a plugin provides them): ${external.join(', ')}');
    }
    return DoctorCheck(id, CheckStatus.ok, '${referenced.length} referenced skills found');
  }

  DoctorCheck _agentsMdSize() {
    const id = 'agents-md-size';
    final file = _file('AGENTS.md');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.skip, 'no AGENTS.md');
    final size = file.lengthSync();
    if (size > agentsMdLimit) {
      return DoctorCheck(id, CheckStatus.warn, 'AGENTS.md is $size bytes, over $agentsMdLimit; some agents truncate it');
    }
    return DoctorCheck(id, CheckStatus.ok, 'AGENTS.md $size of $agentsMdLimit bytes');
  }

  DoctorCheck _gitignore() {
    const id = 'gitignore';
    final file = _file('.gitignore');
    final listed = file.existsSync() && file.readAsLinesSync().any((line) => line.trim() == '.ai/bin/');
    if (listed) return const DoctorCheck(id, CheckStatus.ok, '.ai/bin/ ignored');
    return const DoctorCheck(id, CheckStatus.warn, '.ai/bin/ not in .gitignore; the compiled guard is per machine');
  }

  /// `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`; null when no home directory is known.
  static String? defaultUserSkillsDir() {
    final env = Platform.environment;
    final configDir = env['CLAUDE_CONFIG_DIR'];
    if (configDir != null && configDir.isNotEmpty) return p.join(configDir, 'skills');
    final home = env['HOME'] ?? env['USERPROFILE'];
    return home == null ? null : p.join(home, '.claude', 'skills');
  }
}
