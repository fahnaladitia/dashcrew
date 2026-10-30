import 'dart:io';

import 'package:test/test.dart';

void main() {
  test('dashcrew --help lists usage', () async {
    final result = await Process.run(Platform.resolvedExecutable, ['run', 'bin/dashcrew.dart', '--help']);

    expect(result.exitCode, 0);
    expect(result.stdout, contains('Usage: dashcrew'));
  });
}
