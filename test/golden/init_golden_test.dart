import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Rewrite `expected-project.yaml` instead of comparing: `DASHCREW_UPDATE_GOLDENS=1 dart test test/golden`.
final update = Platform.environment['DASHCREW_UPDATE_GOLDENS'] == '1';

void main() {
  final catalog = Catalog.load('.');
  final fixtureDir = p.join('test', 'golden', 'init-app');
  final expectedFile = File(p.join(fixtureDir, 'expected-project.yaml'));
  late Directory project;

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_golden_init_');
    final input = Directory(p.join(fixtureDir, 'input'));
    for (final file in input.listSync(recursive: true).whereType<File>()) {
      File(p.join(project.path, p.relative(file.path, from: input.path)))
        ..parent.createSync(recursive: true)
        ..writeAsStringSync(file.readAsStringSync());
    }
    Init(catalog: catalog, projectDir: project.path).run(
      yes: true,
      force: false,
      remoteBranches: const ['origin/HEAD', 'origin/develop', 'origin/main'],
      ask: (_) => null,
      out: (_) {},
    );
  });
  tearDown(() => project.deleteSync(recursive: true));

  test('init --yes writes expected-project.yaml', () {
    final actual = File(p.join(project.path, '.ai', 'project.yaml')).readAsStringSync();
    if (update) {
      expectedFile.writeAsStringSync(actual);
      return;
    }
    expect(actual, expectedFile.readAsStringSync(),
        reason: 'if intended: DASHCREW_UPDATE_GOLDENS=1 dart test test/golden, then review the diff');
  });

  test('the written profile syncs and passes doctor besides the uncompiled guard', () async {
    Sync(catalog: catalog, projectDir: project.path, compileGuard: false).run();

    final checks = await Doctor(catalog: catalog, projectDir: project.path, packageRoot: '.').run();

    final failures = [
      for (final check in checks)
        if (check.status == CheckStatus.fail && check.id != 'guard-binary') '${check.id}: ${check.detail}',
    ];
    expect(failures, isEmpty);
  });
}
