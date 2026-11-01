import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late String root;
  late String app;
  late Worktrees worktrees;
  final output = <String>[];

  String git(String dir, List<String> args) {
    final result = Process.runSync('git', ['-c', 'user.name=t', '-c', 'user.email=t@example.com', ...args], workingDirectory: dir);
    if (result.exitCode != 0) fail('git ${args.join(' ')}: ${result.stderr}');
    return (result.stdout as String).trim();
  }

  void write(String path, String content) {
    File(path)
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(content);
  }

  void commitAll(String dir, String message) {
    git(dir, ['add', '-A']);
    git(dir, ['commit', '-q', '-m', message]);
  }

  setUp(() {
    root = Directory.systemTemp.createTempSync('dashcrew_worktree_').resolveSymbolicLinksSync();
    git(root, ['init', '-q', '--bare', '-b', 'main', 'origin.git']);
    git(root, ['clone', '-q', p.join(root, 'origin.git'), 'app']);
    app = p.join(root, 'app');
    write(p.join(app, '.gitignore'), 'AGENTS.md\nCLAUDE.md\n.claude/\n.codex/\n.agents/\n.ai/bin/\ndocs/plans/\n');
    write(p.join(app, '.ai', 'project.yaml'), 'toolkit: 1\nagents: [claude-code]\npaths: { plans: docs/plans }\n');
    write(p.join(app, 'docs', 'readme.md'), 'docs\n');
    write(p.join(app, 'lib', 'a.dart'), 'void a() {}\n');
    commitAll(app, 'init');
    git(app, ['push', '-q', 'origin', 'main']);
    git(app, ['push', '-q', 'origin', 'main:develop']);
    git(app, ['fetch', '-q', 'origin']);
    write(p.join(app, 'AGENTS.md'), 'rules\n');
    write(p.join(app, 'CLAUDE.md'), '@AGENTS.md\n');
    write(p.join(app, '.claude', 'settings.json'), '{}\n');
    write(p.join(app, '.codex', 'hooks.json'), '{}\n');
    write(p.join(app, '.agents', 'skills', 'plan', 'SKILL.md'), 'plan\n');
    write(p.join(app, '.ai', 'bin', 'guard'), 'binary\n');
    write(p.join(app, 'docs', 'plans', 'x', 'plan.md'), 'plan\n');
    output.clear();
    worktrees = Worktrees(projectDir: app);
  });
  tearDown(() => Directory(root).deleteSync(recursive: true));

  String worktreePath(String name) => p.join('$app.worktrees', name);

  group('create', () {
    test('adds a worktree with no upstream, the base recorded and the toolkit linked', () {
      worktrees.create('feat/orders', 'origin/develop', installDeps: false, out: output.add);
      final path = worktreePath('feat-orders');

      expect(git(path, ['branch', '--show-current']), 'feat/orders');
      expect(Process.runSync('git', ['rev-parse', '--abbrev-ref', '@{u}'], workingDirectory: path).exitCode, isNot(0));
      expect(git(app, ['config', '--get', 'branch.feat/orders.dashcrew-base']), 'develop');
      for (final linked in ['AGENTS.md', 'CLAUDE.md', '.claude', '.codex', '.agents', '.ai/bin', 'docs/plans']) {
        expect(FileSystemEntity.isLinkSync(p.join(path, linked)), isTrue, reason: linked);
        expect(Link(p.join(path, linked)).targetSync(), p.join(app, linked));
      }
      expect(FileSystemEntity.isLinkSync(p.join(path, '.ai', 'project.yaml')), isFalse);
      expect(git(path, ['status', '--porcelain']), isEmpty);
      expect(output, contains('upstream: none'));
      expect(output, contains('deps: skipped'));
    });

    test('runs the deps command of the profile platform when one of its detect files exists', () {
      write(p.join(app, '.ai', 'project.yaml'), '''
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: touch deps-ran, format: null } }
''');
      write(p.join(app, 'Makefile'), 'deps:\n');
      commitAll(app, 'platform other');

      Worktrees(projectDir: app, catalog: Catalog.load('.')).create('feat/a', 'main', out: output.add);

      expect(output, contains('deps: ok (touch deps-ran)'));
      expect(File(p.join(worktreePath('feat-a'), 'deps-ran')).existsSync(), isTrue);
    });

    test('deps is skipped without a detect file and reports a failing command with the rerun line', () {
      Worktrees(projectDir: app, catalog: Catalog.load('.')).create('feat/a', 'main', out: output.add);
      expect(output, contains('deps: skipped (no pubspec.yaml)'));

      write(p.join(app, '.ai', 'project.yaml'), '''
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [lib/a.dart], commands: { analyze: a, test: b, test_scoped: c, deps: exit 3 } }
''');
      Worktrees(projectDir: app, catalog: Catalog.load('.')).create('feat/b', 'main', out: output.add);
      expect(output, contains('deps: failed (exit 3); rerun: cd ${worktreePath('feat-b')} && exit 3'));
    });

    test('links only what the main checkout ignores, so nothing it may commit gets hidden', () {
      write(p.join(app, '.mcp.json'), '{}\n');
      git(app, ['rm', '-q', '--cached', '.ai/project.yaml']);
      git(app, ['commit', '-q', '-m', 'untrack profile']);
      git(app, ['push', '-q', 'origin', 'main']);

      worktrees.create('feat/a', 'origin/main', installDeps: false, out: output.add);
      final path = worktreePath('feat-a');

      expect(FileSystemEntity.typeSync(p.join(path, '.mcp.json'), followLinks: false), FileSystemEntityType.notFound);
      expect(FileSystemEntity.typeSync(p.join(path, '.ai', 'project.yaml'), followLinks: false), FileSystemEntityType.notFound);
      expect(FileSystemEntity.isLinkSync(p.join(path, '.ai', 'bin')), isTrue);
      expect(git(path, ['status', '--porcelain']), isEmpty);
      final mainStatus = git(app, ['status', '--porcelain', '--untracked-files=all']);
      expect(mainStatus, contains('?? .mcp.json'));
      expect(mainStatus, contains('?? .ai/project.yaml'));
    });

    test('refuses a missing base, an existing branch and an existing folder', () {
      expect(() => worktrees.create('feat/a', 'origin/nope', installDeps: false, out: output.add),
          throwsA(isA<WorktreeException>().having((e) => e.message, 'message', contains('origin/nope'))));

      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      expect(() => worktrees.create('feat/a', 'main', installDeps: false, out: output.add),
          throwsA(isA<WorktreeException>().having((e) => e.message, 'message', contains('already exists'))));
      expect(() => worktrees.create('feat/b', 'main', name: 'feat-a', installDeps: false, out: output.add),
          throwsA(isA<WorktreeException>().having((e) => e.message, 'message', contains('already exists'))));
    });

    test('works from inside another worktree', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);

      Worktrees(projectDir: worktreePath('feat-a')).create('feat/b', 'main', installDeps: false, out: output.add);

      expect(Directory(worktreePath('feat-b')).existsSync(), isTrue);
    });
  });

  test('link restores a missing link and drops an upstream', () {
    worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
    final path = worktreePath('feat-a');
    Link(p.join(path, 'AGENTS.md')).deleteSync();
    git(path, ['branch', '--set-upstream-to=origin/main']);

    final added = worktrees.link('feat-a', out: output.add);

    expect(added, ['AGENTS.md']);
    expect(Process.runSync('git', ['rev-parse', '--abbrev-ref', '@{u}'], workingDirectory: path).exitCode, isNot(0));
    expect(worktrees.link('feat-a', out: output.add), isEmpty);
  });

  test('list shows branch, base, upstream, dirty lines and missing links', () {
    worktrees.create('feat/a', 'origin/develop', installDeps: false, out: output.add);
    write(p.join(worktreePath('feat-a'), 'lib', 'a.dart'), 'void a() { }\n');
    Link(p.join(worktreePath('feat-a'), 'CLAUDE.md')).deleteSync();

    final info = worktrees.list().single;

    expect(info.name, 'feat-a');
    expect(info.branch, 'feat/a');
    expect(info.base, 'develop');
    expect(info.upstream, isNull);
    expect(info.dirty, 1);
    expect(info.missingLinks, ['CLAUDE.md']);
  });

  group('remove', () {
    test('refuses a dirty worktree', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      write(p.join(worktreePath('feat-a'), 'lib', 'new.dart'), '\n');

      expect(() => worktrees.remove('feat-a', out: output.add),
          throwsA(isA<WorktreeException>().having((e) => e.message, 'message', contains('uncommitted'))));
      expect(Directory(worktreePath('feat-a')).existsSync(), isTrue);
    });

    test('removes a clean worktree, keeps the branch and the linked toolkit', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);

      worktrees.remove('feat-a', out: output.add);

      expect(Directory(worktreePath('feat-a')).existsSync(), isFalse);
      expect(git(app, ['branch', '--list', 'feat/a']), contains('feat/a'));
      expect(File(p.join(app, 'AGENTS.md')).readAsStringSync(), 'rules\n');
      expect(File(p.join(app, 'docs', 'plans', 'x', 'plan.md')).existsSync(), isTrue);
    });
  });

  group('watch', () {
    late String stateDir;
    final prs = <String, String>{};

    Watch watch() => Watch(worktrees: worktrees, stateDir: stateDir, prLookup: (branch) => prs[branch]);

    setUp(() {
      stateDir = p.join(root, 'state');
      prs.clear();
    });

    test('reports first commit and push once each', () {
      worktrees.create('feat/a', 'origin/main', installDeps: false, out: output.add);
      final path = worktreePath('feat-a');
      expect(watch().pass(const []), isEmpty);

      write(p.join(path, 'lib', 'b.dart'), '\n');
      commitAll(path, 'b');
      expect(watch().pass(const []), ['commit feat/a: first commit, 1 ahead of origin/main']);

      git(path, ['push', '-q', 'origin', 'feat/a']);
      final pushed = watch().pass(const []);
      expect(pushed, hasLength(1));
      expect(pushed.single, startsWith('pushed feat/a: '));
      expect(watch().pass(const []), isEmpty);
    });

    test('reports a moved base with the merge-tree result', () {
      worktrees.create('feat/a', 'origin/main', installDeps: false, out: output.add);
      final path = worktreePath('feat-a');
      write(p.join(path, 'lib', 'a.dart'), 'void a() { /* branch */ }\n');
      commitAll(path, 'branch change');
      watch().pass(const []);

      write(p.join(app, 'lib', 'other.dart'), '\n');
      commitAll(app, 'base change');
      git(app, ['push', '-q', 'origin', 'main']);
      final clean = watch().pass(const []);
      expect(clean.first, startsWith('base-moved origin/main: '));
      expect(clean, contains('merge-tree feat/a vs origin/main: CLEAN'));

      write(p.join(app, 'lib', 'a.dart'), 'void a() { /* base */ }\n');
      commitAll(app, 'conflicting base change');
      git(app, ['push', '-q', 'origin', 'main']);
      expect(watch().pass(const []), contains('merge-tree feat/a vs origin/main: CONFLICT lib/a.dart'));
    });

    test('reports growing file overlap between worktrees', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      worktrees.create('feat/b', 'main', installDeps: false, out: output.add);
      write(p.join(worktreePath('feat-a'), 'lib', 'a.dart'), 'void a() { 1; }\n');
      expect(watch().pass(const []), isEmpty);

      write(p.join(worktreePath('feat-b'), 'lib', 'a.dart'), 'void a() { 2; }\n');
      expect(watch().pass(const []), ['overlap feat/a + feat/b: lib/a.dart']);
      expect(watch().pass(const []), isEmpty);
    });

    test('reports PR changes and ends with done when every watched worktree is merged or removed', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      worktrees.create('feat/b', 'main', installDeps: false, out: output.add);
      expect(watch().pass(const []), isEmpty);

      prs['feat/a'] = '#3 OPEN';
      expect(watch().pass(const []), ['pr feat/a: #3 OPEN']);

      prs['feat/a'] = '#3 MERGED';
      worktrees.remove('feat-b', out: output.add);
      expect(watch().pass(const []), ['pr feat/a: #3 MERGED', 'done: 2 worktrees merged or removed']);
    });

    test('run with once stops after one pass', () async {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      prs['feat/a'] = '#5 MERGED';
      final lines = <String>[];

      await watch().run(const [], once: true, interval: const Duration(hours: 1), out: lines.add);

      expect(lines, ['pr feat/a: #5 MERGED', 'done: 1 worktrees merged or removed']);
    });

    test('names limit the watch to those worktrees', () {
      worktrees.create('feat/a', 'main', installDeps: false, out: output.add);
      worktrees.create('feat/b', 'main', installDeps: false, out: output.add);
      prs['feat/b'] = '#4 OPEN';

      expect(watch().pass(const ['feat-a']), isEmpty);
      expect(() => watch().pass(const ['feat-x']), throwsA(isA<WorktreeException>()));
    });
  });
}
