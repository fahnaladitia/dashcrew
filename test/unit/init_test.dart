import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory project;
  final catalog = Catalog.load('.');

  String read(String relative) => File(p.join(project.path, relative)).readAsStringSync();
  void write(String relative, String content) {
    final file = File(p.join(project.path, relative));
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
  }

  ({String profile, List<String> output}) init({
    bool yes = true,
    bool force = false,
    List<String> answers = const [],
    List<String> remoteBranches = const [],
  }) {
    final output = <String>[];
    final pending = [...answers];
    Init(catalog: catalog, projectDir: project.path).run(
      yes: yes,
      force: force,
      remoteBranches: remoteBranches,
      ask: (prompt) {
        output.add(prompt);
        return pending.isEmpty ? null : pending.removeAt(0);
      },
      out: output.add,
    );
    return (profile: read('.ai/project.yaml'), output: output);
  }

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_init_');
    write('pubspec.yaml', '''
name: app
dependencies:
  flutter_bloc: ^9.0.0
  get_it: ^8.0.0
  go_router: ^14.0.0
  dio: ^5.0.0
  http: ^1.0.0
  core: { path: packages/core }
dev_dependencies:
  mocktail: ^1.0.0
''');
    write('lib/main_dev.dart', '');
  });
  tearDown(() => project.deleteSync(recursive: true));

  test('--yes writes the detection and skips a family with two candidates', () {
    final result = init(remoteBranches: ['origin/main', 'origin/develop']);

    expect(result.profile, '''
# Written by dashcrew init. Edit it, then run dashcrew sync.
toolkit: 1
platform: flutter
agents: [claude-code]
stack:
  di: get_it
  router: go_router
  state: bloc
  test: mocktail
project:
  flavors:
    dev: lib/main_dev.dart
  local_packages: [packages/core]
  base_branches: [main, develop]
''');
    expect(result.output.join('\n'), contains('network: skipped, 2 candidates: dio, http'));
    expect(result.output.last, 'next: dashcrew sync');
    Profile.parse(result.profile, knownOptions: catalog.knownOptions());
  });

  test('interactive answers keep, change, pick and drop families', () {
    final families = catalog.knownOptions()['flutter']!.keys.toList()..sort();
    final answers = {
      for (final family in families) family: '',
      'network': 'dio',
      'router': 'getx',
      'test': '-',
      'tracker': 'other linear_cli',
    };

    final result = init(yes: false, answers: ['', for (final family in families) answers[family]!]);

    expect(result.profile, contains('  network: dio\n'));
    expect(result.profile, contains('  router: getx\n'));
    expect(result.profile, contains('  tracker: { use: other, name: linear_cli }\n'));
    expect(result.profile, contains('  state: bloc\n'));
    expect(result.profile, isNot(contains('  test:')));
    expect(result.output, contains('platform [flutter]: '));
    expect(result.profile, contains('platform: flutter\n'));
    expect(result.output, contains('state [bloc]: '));
    expect(result.output, contains('network [dio | http]: '));
    expect(result.output.join('\n'), contains('state: bloc (flutter_bloc in dependencies)'));
  });

  test('an invalid answer asks again with the valid options', () {
    final families = catalog.knownOptions()['flutter']!.keys.toList()..sort();
    final answers = ['', for (final family in families) if (family == 'state') ...['mobx', 'other'] else ''];

    final result = init(yes: false, answers: answers);

    expect(result.output.where((line) => line == 'state [bloc]: '), hasLength(3));
    expect(result.output.join('\n'), contains('expected one of'));
    expect(result.profile, contains('  state: bloc\n'));
  });

  test('detected params are written in the map form', () {
    write('pubspec.yaml', 'name: app\ndependencies:\n  get_it: ^9.0.0\n  injectable: ^3.0.0\n');

    expect(init().profile, contains('  di: { use: get_it, injectable: true }\n'));
  });

  test('refuses an existing profile without --force', () {
    write('.ai/project.yaml', 'toolkit: 1\n');

    expect(() => init(), throwsA(isA<InitException>().having((e) => e.message, 'message', contains('--force'))));
    expect(read('.ai/project.yaml'), 'toolkit: 1\n');
    expect(init(force: true).profile, startsWith('# Written by dashcrew init.'));
  });

  test('a project pack family is offered and an option missing from the catalog becomes other', () {
    write('.ai/packs/architecture/clean/pack.yaml', 'description: clean\n');
    write('pubspec.yaml', 'name: app\ndependencies:\n  get: ^4.0.0\n');
    final flutter = catalog.platforms['flutter']!;
    final withoutGetx = Catalog(
      core: catalog.core,
      packs: catalog.packs,
      roles: catalog.roles,
      skills: catalog.skills,
      workflows: catalog.workflows,
      platforms: {
        'flutter': PlatformSpec(
          name: 'flutter',
          root: flutter.root,
          detect: flutter.detect,
          commands: flutter.commands,
          packs: {...flutter.packs, 'router': {}},
        ),
      },
    );

    final output = <String>[];
    Init(catalog: withoutGetx, projectDir: project.path).run(
      yes: false,
      force: false,
      remoteBranches: const [],
      ask: (prompt) {
        output.add(prompt);
        return null;
      },
      out: output.add,
    );

    expect(output, contains('architecture [-]: '));
    expect(output, contains('router [other get]: '));
    expect(read('.ai/project.yaml'), contains('  router: { use: other, name: get }\n'));
  });

  group('platform', () {
    const goPlatform = 'detect: [go.mod]\ncommands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }\n';

    test('a repo matching no platform is never given flutter: --yes refuses and names other', () {
      File(p.join(project.path, 'pubspec.yaml')).deleteSync();

      expect(
        () => init(),
        throwsA(isA<InitException>().having((e) => e.message, 'message', contains('platform: other'))),
      );
      expect(File(p.join(project.path, '.ai', 'project.yaml')).existsSync(), isFalse);
    });

    test('a repo matching no platform is offered other, and the commands are asked', () {
      File(p.join(project.path, 'pubspec.yaml')).deleteSync();
      write('Makefile', 'lint:\n');

      final result = init(yes: false, answers: ['', 'Makefile', 'make lint', 'make test', '', 'make test-one', 'make deps', '']);

      expect(result.output, contains('platform [other]: '));
      expect(result.output, contains('  required'));
      expect(result.profile, '''
# Written by dashcrew init. Edit it, then run dashcrew sync.
toolkit: 1
platform: other
platform_config:
  detect: [Makefile]
  commands:
    analyze: "make lint"
    test: "make test"
    test_scoped: "make test-one"
    deps: "make deps"
agents: [claude-code]
''');
      final profile = Profile.parse(result.profile, knownOptions: catalog.knownOptions());
      expect(profile.platformConfig!['commands'], containsPair('format', null));
    });

    test('other without the commands stops instead of writing a broken profile', () {
      File(p.join(project.path, 'pubspec.yaml')).deleteSync();

      expect(
        () => init(yes: false, answers: ['', 'Makefile']),
        throwsA(isA<InitException>().having((e) => e.message, 'message', contains('commands.analyze'))),
      );
    });

    test('a project platform is detected and offers its own families', () {
      File(p.join(project.path, 'pubspec.yaml')).deleteSync();
      write('go.mod', 'module example.com/app\n');
      write('.ai/platforms/go/platform.yaml', goPlatform);
      write('.ai/platforms/go/packs/router/chi/pack.yaml', 'description: chi\n');

      final families = Catalog.load('.').withProjectPacks(project.path).knownOptions()['go']!.keys.toList()..sort();
      final result = init(yes: false, answers: ['', for (final family in families) family == 'router' ? 'chi' : '']);

      expect(result.output.first, startsWith('Detected in'));
      expect(result.output, contains('  platform: go (go.mod)'));
      expect(result.output, contains('platform [go]: '));
      expect(result.output, contains('router [-]: '));
      expect(result.output, isNot(contains('state [-]: ')));
      expect(result.profile, contains('platform: go\n'));
      expect(result.profile, contains('  router: chi\n'));
      expect(result.profile, isNot(contains('lib/main_dev.dart')));
    });

    test('two matching platforms are asked; --yes refuses', () {
      write('go.mod', 'module example.com/app\n');
      write('.ai/platforms/go/platform.yaml', goPlatform);

      expect(
        () => init(),
        throwsA(isA<InitException>().having((e) => e.message, 'message', allOf(contains('flutter'), contains('go')))),
      );
      final families = catalog.knownOptions()['flutter']!.keys.toList()..sort();
      final result = init(yes: false, answers: ['', 'flutter', for (final _ in families) '']);

      expect(result.output, contains('platform [flutter | go]: '));
      expect(result.output.where((line) => line == 'platform [flutter | go]: '), hasLength(2));
      expect(result.profile, contains('platform: flutter\n'));
    });
  });
}
