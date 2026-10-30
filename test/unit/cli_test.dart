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
}
