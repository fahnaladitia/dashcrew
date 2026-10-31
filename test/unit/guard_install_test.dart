import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory home;

  setUp(() => home = Directory.systemTemp.createTempSync('dashcrew_home_'));
  tearDown(() => home.deleteSync(recursive: true));

  test('installs one guard per source hash under the home and reuses it', () {
    final installer = GuardInstaller(packageRoot: '.', home: home.path);

    final first = installer.install();

    expect(first.compiled, isTrue);
    expect(first.path, p.join(home.path, 'guard', installer.sourceHash.substring(0, 16), 'guard'));
    final policy = File(p.join(home.path, 'guard.json'))
      ..writeAsStringSync(jsonEncode(GuardPolicy(
        blockedGit: GuardPolicy.alwaysBlockedGit,
        blockFormatters: true,
        plansDir: '.ai/plans',
        roleCapability: const {},
        forbiddenImports: const [],
        localPackages: const [],
      ).toJson()));
    final event = jsonEncode({'tool_name': 'Bash', 'tool_input': {'command': 'git push'}});
    final run = Process.runSync('sh', ['-c', "printf '%s' '$event' | '${first.path}' '${policy.path}'"]);
    expect(run.exitCode, 2, reason: '${run.stderr}');

    final second = installer.install();
    expect(second.compiled, isFalse);
    expect(second.path, first.path);
  }, timeout: const Timeout(Duration(minutes: 3)));

  test('refuses to install a guard whose source differs from the expected hash', () {
    final installer = GuardInstaller(packageRoot: '.', home: home.path);

    expect(
      () => installer.install(expect: 'f' * 64),
      throwsA(isA<GuardInstallException>().having((e) => e.message, 'message', contains('f' * 16))),
    );
    expect(Directory(p.join(home.path, 'guard')).existsSync(), isFalse);
  });

  test('home defaults to DASHCREW_HOME, else ~/.dashcrew', () {
    expect(GuardInstaller.defaultHome({'DASHCREW_HOME': '/x/dc', 'HOME': '/u'}), '/x/dc');
    expect(GuardInstaller.defaultHome({'HOME': '/u'}), p.join('/u', '.dashcrew'));
    expect(GuardInstaller.defaultHome({}), isNull);
  });
}
