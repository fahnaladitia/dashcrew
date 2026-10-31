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
}
