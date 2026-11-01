import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// `sample-migrate-p3/p3/` is the output of the last pre-P4 dashcrew (three agents, plan gate on, storage and
/// design packs), plus files the team wrote: a plan with handoffs, project memory and an AGENTS.md project block.
/// It is frozen: never regenerate it.
void main() {
  final catalog = Catalog.load('.');
  final fixture = p.join('test', 'golden', 'sample-migrate-p3', 'p3');

  const roles = [
    'dart-tdd-guide',
    'design-refiner',
    'flutter-architect',
    'flutter-build-error-resolver',
    'flutter-code-reviewer',
    'flutter-engineer',
    'flutter-planner',
    'flutter-security-reviewer',
    'local-storage-engineer',
    'product-manager',
    'qa-engineer',
    'tech-lead',
    'worktree-monitor',
  ];
  const roleFiles = {'.claude/agents': '.md', '.codex/agents': '.toml', '.agents/agents': '.md'};
  bool teamFile(String path) => path.startsWith('.ai/plans/') || path.startsWith('.ai/memory/') || path == '.ai/project.yaml';

  late Directory project;
  late Directory fresh;
  late Map<String, String> before;
  late SyncResult result;

  Sync sync(Directory dir) => Sync(catalog: catalog, projectDir: dir.path, compileGuard: false);

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_migrate_');
    before = _tree(fixture);
    before.forEach((path, content) {
      File(p.join(project.path, path))
        ..parent.createSync(recursive: true)
        ..writeAsStringSync(content);
    });
    result = sync(project).run();

    fresh = Directory.systemTemp.createTempSync('dashcrew_migrate_fresh_');
    File(p.join((Directory(p.join(fresh.path, '.ai'))..createSync()).path, 'project.yaml'))
        .writeAsStringSync(before['.ai/project.yaml']!);
    sync(fresh).run();
  });
  tearDown(() {
    project.deleteSync(recursive: true);
    fresh.deleteSync(recursive: true);
  });

  test('the fixture is a P3 output with every old role, orchestrate, checkpoint and the plan gate rule', () {
    for (final MapEntry(key: dir, value: extension) in roleFiles.entries) {
      for (final role in roles) {
        expect(before.containsKey('$dir/$role$extension'), isTrue, reason: '$dir/$role$extension');
      }
    }
    expect(before.keys, containsAll(['.claude/skills/orchestrate/SKILL.md', '.agents/skills/checkpoint/SKILL.md']));
    expect(before['AGENTS.md'], contains('dashcrew gate submit plan'));
  });

  test('sync removes every old role file per agent, orchestrate, checkpoint and the plan gate rule', () {
    final after = _tree(project.path);

    for (final MapEntry(key: dir, value: extension) in roleFiles.entries) {
      for (final role in roles) {
        expect(after.containsKey('$dir/$role$extension'), isFalse, reason: '$dir/$role$extension');
        expect(result.removed, contains('$dir/$role$extension'));
      }
      final left = after.keys.where((path) => path.startsWith('$dir/')).toList()..sort();
      expect(left, ['$dir/explorer$extension', '$dir/reviewer$extension']);
    }
    for (final skill in ['orchestrate', 'checkpoint']) {
      for (final dir in ['.claude/skills', '.agents/skills']) {
        expect(after.containsKey('$dir/$skill/SKILL.md'), isFalse, reason: '$dir/$skill');
      }
    }
    expect(after['AGENTS.md'], isNot(contains('dashcrew gate')));
    expect(after['AGENTS.md'], isNot(contains('Plan gate')));
    expect(after['.ai/guard.json'], isNot(contains('plan_gate')));
  });

  test('files the team wrote are untouched', () {
    final after = _tree(project.path);

    final team = before.keys.where(teamFile).toList()..sort();
    expect(team, containsAll(['.ai/plans/todo-filter/handoff/00-roster.md', '.ai/memory/router-default-transition.md']));
    for (final path in team) {
      expect(after[path], before[path], reason: path);
    }
    String block(String agentsMd) =>
        agentsMd.substring(agentsMd.indexOf(Sync.projectBegin), agentsMd.indexOf(Sync.projectEnd) + Sync.projectEnd.length);
    expect(block(after['AGENTS.md']!), block(before['AGENTS.md']!));
    expect(block(after['AGENTS.md']!), contains('Every list screen has an empty state'));
  });

  test('the migrated project equals a fresh sync of the same profile, besides the team files', () {
    final after = _tree(project.path)..removeWhere((path, _) => teamFile(path));
    final expected = _tree(fresh.path)..removeWhere((path, _) => teamFile(path));

    expect(after.keys.toList()..sort(), expected.keys.toList()..sort());
    for (final path in expected.keys) {
      if (path == 'AGENTS.md') {
        expect(Sync.fileHash(path, after[path]!), Sync.fileHash(path, expected[path]!));
      } else {
        expect(after[path], expected[path], reason: path);
      }
    }
    expect(jsonDecode(after['.ai/lock.json']!), jsonDecode(expected['.ai/lock.json']!));
  });

  test('sync prints the P4 migration note once', () {
    expect(result.notes, [Sync.p4MigrationNote]);
    expect(result.warnings, contains(Profile.planGateRemoved));

    final again = sync(project).run();

    expect(again.notes, isEmpty);
    expect(again.written, isEmpty);
    expect(again.removed, isEmpty);
  });

  test('doctor reports the leftover roster and the plan_gate key, and nothing fails', () async {
    final checks = {
      for (final check in await Doctor(catalog: catalog, projectDir: project.path, machineChecks: false).run()) check.id: check,
    };

    expect(checks['profile']!.status, CheckStatus.warn);
    expect(checks['profile']!.detail, contains(Profile.planGateRemoved));
    expect(checks['handoff-rosters']!.status, CheckStatus.info);
    expect(checks['handoff-rosters']!.detail, contains('.ai/plans/todo-filter/handoff/00-roster.md'));
    expect([for (final check in checks.values) if (check.status == CheckStatus.fail) '${check.id}: ${check.detail}'], isEmpty);
  });
}

/// Relative path (always `/`) -> content for every file under [root].
Map<String, String> _tree(String root) => {
      for (final file in Directory(root).listSync(recursive: true).whereType<File>())
        p.relative(file.path, from: root).replaceAll(r'\', '/'): file.readAsStringSync(),
    };
