import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import '../adapters/claude_code.dart';
import '../adapters/codex.dart';
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
  const Doctor({required this.catalog, required this.projectDir, this.packageRoot, this.userSkillsDir, this.codexHome});

  final Catalog catalog;
  final String projectDir;

  /// dashcrew package root; without it the guard staleness check is skipped.
  final String? packageRoot;

  /// Where user-level skills live; defaults to `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`.
  final String? userSkillsDir;

  /// Codex user config folder; defaults to `$CODEX_HOME`, else `~/.codex`.
  final String? codexHome;

  static const agentsMdLimit = 32 * 1024;
  static final _skillReference = RegExp(r'`([a-z0-9][a-z0-9-]*)` skill|skill `([a-z0-9][a-z0-9-]*)`');

  /// Every check, with the project packs in `.ai/packs/` overlaid on [catalog] when they parse.
  Future<List<DoctorCheck>> run() async {
    final Catalog overlaid;
    try {
      overlaid = catalog.withProjectPacks(projectDir);
    } on CatalogException catch (error) {
      return _checks(DoctorCheck('project-packs', CheckStatus.fail, error.toString()));
    }
    final doctor = Doctor(
      catalog: overlaid,
      projectDir: projectDir,
      packageRoot: packageRoot,
      userSkillsDir: userSkillsDir,
      codexHome: codexHome,
    );
    return doctor._checks(doctor._projectPacks());
  }

  Future<List<DoctorCheck>> _checks(DoctorCheck projectPacks) async {
    final profile = _profileCheck();
    final (lockCheck, lock) = _lockCheck();
    final lockedFiles = (lock?['files'] as Map?)?.cast<String, String>();
    final agents = _agents();
    return [
      profile,
      projectPacks,
      lockCheck,
      _drift(lockedFiles),
      _stale(lockedFiles, profileOk: profile.status == CheckStatus.ok),
      _guardPolicy(),
      await _guardBinary(lock),
      if (agents.contains('claude-code')) ...[
        _hook(),
        _claudeMd(),
        _mcp(),
        _skills(lockedFiles),
      ],
      if (agents.contains('codex')) ...[
        _codexHook(),
        _codexMcp(),
        _codexSkillsBudget(),
        _codexTrust(),
      ],
      _agentsMdSize(),
      _gitignore(),
    ];
  }

  File _file(String relative) => File(p.join(projectDir, relative));

  /// Agents of the profile; Claude Code alone when the profile cannot be read, as before Codex support.
  Set<String> _agents() {
    try {
      return Profile.parse(_file('.ai/project.yaml').readAsStringSync(), knownOptions: catalog.knownOptions()).agents.toSet();
    } catch (_) {
      return {'claude-code'};
    }
  }

  DoctorCheck _codexHook() {
    const id = 'codex-hook';
    final file = _file('.codex/hooks.json');
    if (!file.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '.codex/hooks.json missing; run dashcrew sync');
    final List<Object?> preToolUse;
    try {
      final config = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
      preToolUse = ((config['hooks'] as Map?)?['PreToolUse'] as List?) ?? const [];
    } catch (error) {
      return DoctorCheck(id, CheckStatus.fail, '.codex/hooks.json invalid: $error');
    }
    if (preToolUse.any((entry) => jsonEncode(entry) == jsonEncode(CodexAdapter.guardHook))) {
      return const DoctorCheck(id, CheckStatus.ok, 'PreToolUse guard hook present; Codex runs it once approved in /hooks');
    }
    final edited = preToolUse.any((entry) => jsonEncode(entry).contains('.ai/bin/guard'));
    return DoctorCheck(id, CheckStatus.fail,
        '${edited ? 'PreToolUse guard hook differs from what sync writes' : 'PreToolUse guard hook missing'}; run dashcrew sync');
  }

  DoctorCheck _codexMcp() {
    const id = 'codex-mcp';
    final file = _file('.codex/config.toml');
    final toml = file.existsSync() ? file.readAsStringSync() : '';
    final missing = [
      for (final name in CodexAdapter.mcpServers.keys)
        if (!CodexAdapter.definesMcpServer(toml, name)) name,
    ];
    if (missing.isNotEmpty) {
      return DoctorCheck(id, CheckStatus.warn, '.codex/config.toml does not define ${missing.join(', ')}; run dashcrew sync');
    }
    final byUser = !toml.contains('# dashcrew:begin');
    return DoctorCheck(id, CheckStatus.ok,
        'defined: ${CodexAdapter.mcpServers.keys.join(', ')}${byUser ? ' (by the user, outside the dashcrew block)' : ''}');
  }

  DoctorCheck _codexSkillsBudget() {
    const id = 'codex-skills-budget';
    final dir = Directory(p.join(projectDir, '.agents', 'skills'));
    final files = !dir.existsSync()
        ? <File>[]
        : [
            for (final sub in dir.listSync().whereType<Directory>())
              if (File(p.join(sub.path, 'SKILL.md')).existsSync()) File(p.join(sub.path, 'SKILL.md')),
          ];
    final size = CodexAdapter.skillListChars(files.map((f) => f.readAsStringSync()));
    const budget = CodexAdapter.skillListBudget;
    if (size > budget) {
      return DoctorCheck(id, CheckStatus.warn, 'skill list is $size characters, over $budget; Codex may drop descriptions');
    }
    return DoctorCheck(id, CheckStatus.ok, '${files.length} skills, list $size of $budget characters');
  }

  static final _projectTable = RegExp(r'''^\s*\[projects\.(?:"([^"]+)"|'([^']+)')\]\s*$''', multiLine: true);
  static final _trusted = RegExp(r'''^\s*trust_level\s*=\s*["']trusted["']''', multiLine: true);

  /// Codex loads `.codex/` only for a trusted project; trust is set by Codex itself, never by dashcrew.
  DoctorCheck _codexTrust() {
    const id = 'codex-trust';
    final home = codexHome ?? defaultCodexHome();
    final config = home == null ? null : File(p.join(home, 'config.toml'));
    final toml = config != null && config.existsSync() ? config.readAsStringSync() : '';
    final trusted = <String>[];
    for (final match in _projectTable.allMatches(toml)) {
      final rest = toml.substring(match.end);
      final next = RegExp(r'^\s*\[', multiLine: true).firstMatch(rest);
      if (_trusted.hasMatch(next == null ? rest : rest.substring(0, next.start))) trusted.add(match.group(1) ?? match.group(2)!);
    }
    final here = {projectDir, if (Directory(projectDir).existsSync()) Directory(projectDir).resolveSymbolicLinksSync()};
    if (trusted.any((path) => here.any((dir) => dir == path || p.isWithin(path, dir)))) {
      return const DoctorCheck(id, CheckStatus.ok, 'project trusted in the Codex config');
    }
    return const DoctorCheck(id, CheckStatus.warn,
        'project not trusted in the Codex config, so .codex/ is ignored; trust it when Codex asks, then approve the guard in /hooks');
  }

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

  DoctorCheck _projectPacks() {
    const id = 'project-packs';
    final names = [
      for (final options in catalog.packs.values)
        for (final pack in options.values)
          if (pack.fromProject) '${pack.family}/${pack.option}',
    ]..sort();
    if (names.isEmpty) return const DoctorCheck(id, CheckStatus.ok, 'no project packs in .ai/packs');

    Profile? profile;
    try {
      profile = Profile.parse(_file('.ai/project.yaml').readAsStringSync(), knownOptions: catalog.knownOptions());
    } catch (_) {
      profile = null;
    }
    final unused = [
      if (profile != null)
        for (final name in names)
          if (profile.stack[name.split('/').first]?.option != name.split('/').last) name,
    ];
    if (unused.isNotEmpty) return DoctorCheck(id, CheckStatus.warn, 'not selected in .ai/project.yaml: ${unused.join(', ')}');
    return DoctorCheck(id, CheckStatus.ok, '${names.length} project packs: ${names.join(', ')}');
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

  /// `$CODEX_HOME`, else `~/.codex`; null when no home directory is known.
  static String? defaultCodexHome() {
    final env = Platform.environment;
    final codexHome = env['CODEX_HOME'];
    if (codexHome != null && codexHome.isNotEmpty) return codexHome;
    final home = env['HOME'] ?? env['USERPROFILE'];
    return home == null ? null : p.join(home, '.codex');
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
