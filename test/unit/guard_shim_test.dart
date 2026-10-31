import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Runs the generated `.ai/bin/guard` shim through `sh`, with a stub in place of the compiled guard.
void main() {
  late Directory root;
  late String shim;
  late String home;
  const hash = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';

  String stubPath() => p.join(home, 'guard', hash.substring(0, 16), 'guard');

  void writeExecutable(String path, String content) {
    File(path)
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(content);
    Process.runSync('chmod', ['+x', path]);
  }

  Future<ProcessResult> runShim(String input, {String path = '/usr/bin:/bin'}) async {
    final process = await Process.start(shim, ['policy.json'], environment: {'DASHCREW_HOME': home, 'PATH': path, 'HOME': root.path});
    process.stdin.write(input);
    await process.stdin.close();
    final out = await process.stdout.transform(const SystemEncoding().decoder).join();
    final err = await process.stderr.transform(const SystemEncoding().decoder).join();
    return ProcessResult(process.pid, await process.exitCode, out, err);
  }

  setUp(() {
    root = Directory(Directory.systemTemp.createTempSync('dashcrew_shim_').resolveSymbolicLinksSync());
    home = p.join(root.path, 'home');
    shim = p.join(root.path, 'guard-shim');
    writeExecutable(shim, GuardInstaller.shim(hash));
  });
  tearDown(() => root.deleteSync(recursive: true));

  test('hands stdin and arguments to the installed guard and keeps its exit code', () async {
    writeExecutable(stubPath(), '#!/bin/sh\necho "args \$*"\ncat\nexit 2\n');

    final result = await runShim('{"tool_name":"Bash"}');

    expect(result.exitCode, 2);
    expect(result.stdout, 'args policy.json\n{"tool_name":"Bash"}');
  });

  test('without the guard or dashcrew it says so on stderr and allows the call', () async {
    final result = await runShim('{"tool_name":"Bash"}');

    expect(result.exitCode, 0);
    expect(result.stdout, isEmpty);
    expect(result.stderr, contains('not installed on this machine'));
  });

  test('installs the expected guard on demand through dashcrew on PATH, then runs it', () async {
    final bin = p.join(root.path, 'bin');
    writeExecutable(
      p.join(bin, 'dashcrew'),
      '#!/bin/sh\necho "\$*" > "${p.join(root.path, 'install-args')}"\n'
      'mkdir -p "\$(dirname "${stubPath()}")"\nprintf \'#!/bin/sh\\ncat > /dev/null\\nexit 2\\n\' > "${stubPath()}"\nchmod +x "${stubPath()}"\n',
    );

    final result = await runShim('{}', path: '$bin:/usr/bin:/bin');

    expect(result.exitCode, 2, reason: '${result.stderr}');
    expect(File(p.join(root.path, 'install-args')).readAsStringSync().trim(), 'guard install --expect $hash');
  });
}
