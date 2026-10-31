import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory project;

  void write(String relative, String content) {
    final file = File(p.join(project.path, relative));
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
  }

  Detection detectWith(Map<String, String> dependencies, {Map<String, String> devDependencies = const {}}) {
    final deps = dependencies.entries.map((e) => '  ${e.key}: ${e.value}').join('\n');
    final devDeps = devDependencies.entries.map((e) => '  ${e.key}: ${e.value}').join('\n');
    write('pubspec.yaml', 'name: app\ndependencies:\n  flutter: { sdk: flutter }\n$deps\ndev_dependencies:\n$devDeps\n');
    return detect(project.path, remoteBranches: const []);
  }

  setUp(() => project = Directory.systemTemp.createTempSync('dashcrew_detect_'));
  tearDown(() => project.deleteSync(recursive: true));

  final rows = {
    'flutter_bloc': 'state: bloc',
    'bloc': 'state: bloc',
    'get_it': 'di: get_it',
    'get': 'router: getx',
    'dio': 'network: dio',
    'hive': 'storage: hive',
    'hive_ce': 'storage: hive',
    'hive_flutter': 'storage: hive',
    'posthog_flutter': 'analytics: posthog',
    'flutter_riverpod': 'state: riverpod',
    'riverpod': 'state: riverpod',
    'hooks_riverpod': 'state: riverpod',
    'provider': 'state: provider',
    'mobx': 'state: other mobx',
    'go_router': 'router: other go_router',
    'auto_route': 'router: other auto_route',
    'http': 'network: other http',
    'chopper': 'network: other chopper',
    'isar': 'storage: other isar',
    'drift': 'storage: other drift',
    'sqflite': 'storage: other sqflite',
  };
  rows.forEach((package, expected) {
    test('$package detects $expected', () {
      final detection = detectWith({package: '^1.0.0'});
      final family = expected.split(':').first;

      expect(detection.stack.keys, [family]);
      expect('$family: ${detection.stack[family]!.single.label}', expected);
      expect(detection.stack[family]!.single.evidence, ['$package in dependencies']);
    });
  });

  for (final package in ['mocktail', 'mockito']) {
    test('$package in dev_dependencies detects test: other $package', () {
      final detection = detectWith({}, devDependencies: {package: '^1.0.0'});

      expect(detection.stack['test']!.single.label, 'other $package');
      expect(detection.stack['test']!.single.evidence, ['$package in dev_dependencies']);
    });
  }

  test('two packages of one library give one candidate with both as evidence', () {
    final detection = detectWith({'flutter_bloc': '^9.0.0', 'bloc': '^9.0.0'});

    expect(detection.stack['state']!.single.option, 'bloc');
    expect(detection.stack['state']!.single.evidence, ['bloc in dependencies', 'flutter_bloc in dependencies']);
  });

  test('two libraries for one family give two candidates', () {
    final detection = detectWith({'dio': '^5.0.0', 'http': '^1.0.0'});

    expect(detection.stack['network']!.map((c) => c.label), ['dio', 'other http']);
  });

  test('unknown packages are ignored', () {
    expect(detectWith({'intl': 'any', 'equatable': 'any'}).stack, isEmpty);
  });

  test('path dependencies inside the project become local packages', () {
    write('pubspec.yaml', '''
name: app
dependencies:
  core: { path: packages/core/ }
  shared:
    path: ./shared
  outside: { path: ../outside }
''');

    expect(detect(project.path, remoteBranches: const []).localPackages, ['packages/core', 'shared']);
  });

  test('lib/main_<flavor>.dart files become flavors', () {
    write('pubspec.yaml', 'name: app\n');
    write('lib/main.dart', '');
    write('lib/main_dev.dart', '');
    write('lib/main_prod.dart', '');

    expect(detect(project.path, remoteBranches: const []).flavors, {'dev': 'lib/main_dev.dart', 'prod': 'lib/main_prod.dart'});
  });

  test('known remote branches become base branches in a fixed order', () {
    write('pubspec.yaml', 'name: app\n');

    final detection = detect(project.path, remoteBranches: ['origin/feature/x', 'origin/develop', 'origin/main', 'origin/HEAD']);

    expect(detection.baseBranches, ['main', 'develop']);
  });

  test('missing pubspec.yaml fails', () {
    expect(() => detect(project.path, remoteBranches: const []), throwsA(isA<InitException>()));
  });
}
