import 'dart:io';

import 'package:test/test.dart';

void main() {
  test('dashcrew --help lists usage', () async {
    final result = await Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', '--help']);

    expect(result.exitCode, 0);
    expect(result.stdout, contains('Usage: dashcrew'));
  });

  test('dashcrew doctor exits 1 on a project that was never synced', () async {
    final project = Directory.systemTemp.createTempSync('dashcrew_cli_');
    addTearDown(() => project.deleteSync(recursive: true));

    final result =
        await Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', 'doctor', '--project', project.path]);

    expect(result.exitCode, 1);
    expect(result.stdout, contains('fail  profile'));
    expect(result.stdout, contains('fail  lock'));
  });

  test('dashcrew init --yes writes the profile, and a second run refuses with exit 1', () async {
    final project = Directory.systemTemp.createTempSync('dashcrew_cli_');
    addTearDown(() => project.deleteSync(recursive: true));
    File('${project.path}/pubspec.yaml').writeAsStringSync('name: app\ndependencies:\n  dio: ^5.0.0\n');
    final args = ['run', 'bin/dashcrew.dart', 'init', '--project', project.path, '--yes'];

    final first = await Process.run(Platform.resolvedExecutable, args);

    expect(first.exitCode, 0, reason: '${first.stderr}');
    expect(first.stdout, contains('next: dashcrew sync'));
    expect(File('${project.path}/.ai/project.yaml').readAsStringSync(), contains('  network: dio\n'));

    final second = await Process.run(Platform.resolvedExecutable, args);

    expect(second.exitCode, 1);
    expect(second.stderr, contains('--force'));
  });

  test('dashcrew worktree create, list and remove', () async {
    final root = Directory.systemTemp.createTempSync('dashcrew_cli_').resolveSymbolicLinksSync();
    addTearDown(() => Directory(root).deleteSync(recursive: true));
    final app = '$root/app';
    for (final args in [
      ['init', '-q', '-b', 'main', app],
      ['-C', app, '-c', 'user.name=t', '-c', 'user.email=t@example.com', 'commit', '-q', '--allow-empty', '-m', 'init'],
    ]) {
      expect((await Process.run('git', args)).exitCode, 0);
    }
    Future<ProcessResult> dashcrew(List<String> args) =>
        Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', 'worktree', ...args, '--project', app]);

    final create = await dashcrew(['create', 'feat/a', 'main', '--no-pub']);
    expect(create.exitCode, 0, reason: '${create.stderr}');
    expect(create.stdout, contains('upstream: none'));

    final list = await dashcrew(['list']);
    expect(list.stdout, contains('feat-a  feat/a  base main  upstream -  dirty 0'));

    final missing = await dashcrew(['remove', 'nope']);
    expect(missing.exitCode, 1);
    expect(missing.stderr, contains('No worktree named nope'));

    expect((await dashcrew(['remove', 'feat-a'])).exitCode, 0);
  });

  test('dashcrew gate status without a linked issue, and submit without --file', () async {
    final repo = Directory.systemTemp.createTempSync('dashcrew_cli_gate_');
    addTearDown(() => repo.deleteSync(recursive: true));
    Process.runSync('git', ['init', '-q', '-b', 'feat/x'], workingDirectory: repo.path);

    final status = await Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', 'gate', 'status', '--project', repo.path]);
    expect(status.exitCode, 0, reason: '${status.stderr}');
    expect(status.stdout, contains('plan: no issue linked'));

    final submit = await Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', 'gate', 'submit', 'plan', '--project', repo.path]);
    expect(submit.exitCode, 64);
    expect(submit.stderr, contains('file'));
  });
}
