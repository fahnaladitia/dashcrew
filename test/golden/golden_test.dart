import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Rewrite `expected/` from the current output instead of comparing: `DASHCREW_UPDATE_GOLDENS=1 dart test test/golden`.
final update = Platform.environment['DASHCREW_UPDATE_GOLDENS'] == '1';

void main() {
  final catalog = Catalog.load('.');

  for (final fixture in ['sample-bloc-getx', 'sample-project-pack', 'sample-riverpod-gorouter', 'sample-codex']) {
    group(fixture, () {
      final fixtureDir = p.join('test', 'golden', fixture);
      final expectedDir = p.join(fixtureDir, 'expected');
      late Directory project;

      Sync sync() => Sync(catalog: catalog, projectDir: project.path, compileGuard: false);

      setUp(() {
        project = Directory.systemTemp.createTempSync('dashcrew_golden_');
        File(p.join(fixtureDir, 'project.yaml')).copySync(p.join((Directory(p.join(project.path, '.ai'))..createSync()).path, 'project.yaml'));
        _tree(p.join(fixtureDir, 'packs')).forEach((path, content) {
          File(p.join(project.path, '.ai', 'packs', path))
            ..parent.createSync(recursive: true)
            ..writeAsStringSync(content);
        });
        sync().run();
      });
      tearDown(() => project.deleteSync(recursive: true));

      test('sync output matches expected/', () {
        final actual = _tree(project.path)
          ..remove('.ai/project.yaml')
          ..removeWhere((path, _) => path.startsWith('.ai/packs/'));
        if (update) {
          final dir = Directory(expectedDir);
          if (dir.existsSync()) dir.deleteSync(recursive: true);
          actual.forEach((path, content) {
            File(p.join(expectedDir, path))
              ..parent.createSync(recursive: true)
              ..writeAsStringSync(content);
          });
          return;
        }

        final expected = _tree(expectedDir);
        const hint = 'if the change is intended: DASHCREW_UPDATE_GOLDENS=1 dart test test/golden, then review the diff';
        expect(actual.keys.toList()..sort(), expected.keys.toList()..sort(), reason: 'file set differs; $hint');
        final differing = [for (final path in expected.keys) if (actual[path] != expected[path]) path]..sort();
        expect(differing, isEmpty, reason: 'content differs; $hint');
      });

      test('second sync writes nothing', () {
        final result = sync().run();

        expect(result.written, isEmpty);
        expect(result.removed, isEmpty);
      });

      test('doctor finds no failure besides the uncompiled guard', () async {
        final checks = await Doctor(catalog: catalog, projectDir: project.path, packageRoot: '.').run();

        final failures = [
          for (final check in checks)
            if (check.status == CheckStatus.fail && check.id != 'guard-binary') '${check.id}: ${check.detail}',
        ];
        expect(failures, isEmpty);
      });
    });
  }
}

/// Relative path (always `/`) -> content for every file under [root]; empty when [root] does not exist.
Map<String, String> _tree(String root) => !Directory(root).existsSync() ? {} : {
      for (final file in Directory(root).listSync(recursive: true).whereType<File>())
        p.relative(file.path, from: root).replaceAll(r'\', '/'): file.readAsStringSync(),
    };
