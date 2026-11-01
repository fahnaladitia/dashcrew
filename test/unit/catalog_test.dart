import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory root;

  void write(String relative, String content) {
    final file = File(p.join(root.path, relative));
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
  }

  setUp(() => root = Directory.systemTemp.createTempSync('dashcrew_catalog_'));
  tearDown(() => root.deleteSync(recursive: true));

  test('loads core fragments with front matter', () {
    write('core/git/commits.md', '''
---
target: agents_md
order: 20
rules: [git.no-agent-commit]
when: { policy.agent_commit: false }
---
Never commit.
''');

    final catalog = Catalog.load(root.path);

    expect(catalog.core, hasLength(1));
    final fragment = catalog.core.single;
    expect(fragment.target, 'agents_md');
    expect(fragment.order, 20);
    expect(fragment.rules, ['git.no-agent-commit']);
    expect(fragment.when, {'policy.agent_commit': false});
    expect(fragment.body.trim(), 'Never commit.');
    expect(fragment.source, p.join('core', 'git', 'commits.md'));
  });

  test('order defaults to 100 and rules to empty', () {
    write('core/a.md', '---\ntarget: agents_md\n---\nA\n');

    final fragment = Catalog.load(root.path).core.single;

    expect(fragment.order, 100);
    expect(fragment.rules, isEmpty);
    expect(fragment.when, isEmpty);
  });

  test('fragment without target fails with its file', () {
    write('core/broken.md', '---\norder: 1\n---\nx\n');

    expect(
      () => Catalog.load(root.path),
      throwsA(isA<CatalogException>().having((e) => e.source, 'source', p.join('core', 'broken.md'))),
    );
  });

  test('fragment without front matter fails', () {
    write('core/plain.md', 'no front matter\n');

    expect(() => Catalog.load(root.path), throwsA(isA<CatalogException>()));
  });

  test('loads packs with params and fragments', () {
    write('packs/state/bloc/pack.yaml', '''
description: flutter_bloc state management
params:
  reference_folder: lib/ui/example/blocs
''');
    write('packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nUse Bloc.\n');
    write('packs/state/riverpod/pack.yaml', 'description: riverpod\n');

    final catalog = Catalog.load(root.path);

    expect(catalog.packs.keys, containsAll(['state']));
    final bloc = catalog.packs['state']!['bloc']!;
    expect(bloc.family, 'state');
    expect(bloc.option, 'bloc');
    expect(bloc.params, {'reference_folder': 'lib/ui/example/blocs'});
    expect(bloc.fragments.single.body.trim(), 'Use Bloc.');
    expect(catalog.packs['state']!['riverpod']!.fragments, isEmpty);
    expect(catalog.knownOptions()['flutter'], {'state': {'bloc', 'riverpod'}});
  });

  test('skill meta.yaml reads review, false when absent', () {
    write('core/skills/security/meta.yaml', 'description: Security checklist.\nwhen_to_use: On auth changes.\nreview: true\n');
    write('core/skills/branching/meta.yaml', 'description: Branch setup.\n');

    final catalog = Catalog.load(root.path);

    expect(catalog.skills['security']!.review, isTrue);
    expect(catalog.skills['branching']!.review, isFalse);
  });

  test('skill meta.yaml with a review that is not a boolean fails with its file', () {
    write('core/skills/security/meta.yaml', 'description: Security checklist.\nreview: yes please\n');

    expect(
      () => Catalog.load(root.path),
      throwsA(isA<CatalogException>().having((e) => e.toString(), 'message', contains('core/skills/security/meta.yaml'))),
    );
  });

  test('pack folder without pack.yaml fails', () {
    write('packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nx\n');

    expect(() => Catalog.load(root.path), throwsA(isA<CatalogException>()));
  });

  group('project packs', () {
    late Directory project;

    void writeProject(String relative, String content) {
      final file = File(p.join(project.path, relative));
      file.parent.createSync(recursive: true);
      file.writeAsStringSync(content);
    }

    setUp(() {
      project = Directory.systemTemp.createTempSync('dashcrew_project_packs_');
      write('packs/state/bloc/pack.yaml', 'description: bloc\n');
      write('packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nUse Bloc.\n');
      write('packs/state/riverpod/pack.yaml', 'description: riverpod\n');
    });
    tearDown(() => project.deleteSync(recursive: true));

    test('replace a built-in option and add a family', () {
      writeProject('.ai/packs/state/bloc/pack.yaml', 'description: team bloc\nparams: { dir: lib/blocs }\n');
      writeProject('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nTeam Bloc.\n');
      writeProject('.ai/packs/architecture/clean/pack.yaml', 'description: clean\n');
      writeProject('.ai/packs/architecture/clean/rules.md', '---\ntarget: agents_md\n---\nClean layers.\n');
      final builtin = Catalog.load(root.path);

      final catalog = builtin.withProjectPacks(project.path);

      final bloc = catalog.packs['state']!['bloc']!;
      expect(bloc.fromProject, isTrue);
      expect(bloc.params, {'dir': 'lib/blocs'});
      expect(bloc.fragments.single.body.trim(), 'Team Bloc.');
      expect(bloc.fragments.single.source, '.ai/packs/state/bloc/rules.md');
      expect(catalog.packs['state']!['riverpod']!.fromProject, isFalse);
      expect(catalog.knownOptions()['flutter'], {'state': {'bloc', 'riverpod'}, 'architecture': {'clean'}});
      expect(builtin.packs['state']!['bloc']!.fromProject, isFalse);
      expect(builtin.packs.keys, ['state']);
    });

    test('no .ai/packs keeps the built-in packs', () {
      final catalog = Catalog.load(root.path).withProjectPacks(project.path);

      expect(catalog.knownOptions()['flutter'], {'state': {'bloc', 'riverpod'}});
    });

    test('broken project pack fails with its project path', () {
      writeProject('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nx\n');

      expect(
        () => Catalog.load(root.path).withProjectPacks(project.path),
        throwsA(isA<CatalogException>().having((e) => e.source, 'source', '.ai/packs/state/bloc/pack.yaml')),
      );
    });

    test('option named other is reserved', () {
      writeProject('.ai/packs/state/other/pack.yaml', 'description: x\n');

      expect(
        () => Catalog.load(root.path).withProjectPacks(project.path),
        throwsA(isA<CatalogException>().having((e) => e.message, 'message', contains('reserved'))),
      );
    });
  });

  group('platforms', () {
    const flutterYaml = '''
description: Flutter apps
detect: [pubspec.yaml]
commands: { analyze: flutter analyze, test: flutter test, test_scoped: flutter test --reporter expanded, deps: flutter pub get, format: dart format }
link: pubspec_overrides.yaml
doc_comment: Dartdoc
mcp_servers:
  lang-server: { command: lang, args: [mcp] }
''';
    late Directory project;

    void writeProject(String relative, String content) {
      final file = File(p.join(project.path, relative));
      file.parent.createSync(recursive: true);
      file.writeAsStringSync(content);
    }

    setUp(() {
      project = Directory.systemTemp.createTempSync('dashcrew_project_platforms_');
      write('core/rules/code.md', '---\ntarget: agents_md\norder: 50\n---\nGeneric code.\n');
      write('core/rules/testing.md', '---\ntarget: agents_md\norder: 80\n---\nGeneric testing.\n');
      write('core/skills/docs/meta.yaml', 'description: generic docs\n');
      write('packs/tracker/none/pack.yaml', 'description: no tracker\n');
      write('platforms/flutter/platform.yaml', flutterYaml);
      write('platforms/flutter/rules/lifecycle.md', '---\ntarget: agents_md\norder: 71\n---\nDispose controllers.\n');
      write('platforms/flutter/rules/testing.md', '---\ntarget: agents_md\norder: 80\n---\nRun flutter test.\n');
      write('platforms/flutter/skills/docs/meta.yaml', 'description: Dartdoc details\n');
      write('platforms/flutter/skills/perf/meta.yaml', 'description: widget performance\n');
      write('platforms/flutter/skills/perf/perf.md', '---\ntarget: skill/perf\n---\nUse const.\n');
      write('platforms/flutter/packs/state/bloc/pack.yaml', 'description: bloc\n');
    });
    tearDown(() => project.deleteSync(recursive: true));

    test('loads platform.yaml, fragments, metas and packs of a built-in platform', () {
      final platform = Catalog.load(root.path).platforms['flutter']!;

      expect(platform.name, 'flutter');
      expect(platform.description, 'Flutter apps');
      expect(platform.detect, ['pubspec.yaml']);
      expect(platform.commands, {
        'analyze': 'flutter analyze',
        'test': 'flutter test',
        'test_scoped': 'flutter test --reporter expanded',
        'deps': 'flutter pub get',
        'format': 'dart format',
      });
      expect(platform.link, 'pubspec_overrides.yaml');
      expect(platform.docComment, 'Dartdoc');
      expect(platform.mcpServers.keys, ['lang-server']);
      expect(platform.mcpServers['lang-server']!.command, 'lang');
      expect(platform.mcpServers['lang-server']!.args, ['mcp']);
      expect(platform.fromProject, isFalse);
      expect(platform.fragments.map((f) => f.source), [
        p.join('platforms', 'flutter', 'rules', 'lifecycle.md'),
        p.join('platforms', 'flutter', 'rules', 'testing.md'),
        p.join('platforms', 'flutter', 'skills', 'perf', 'perf.md'),
      ]);
      expect(platform.skills.keys, ['docs', 'perf']);
      expect(platform.packs['state']!['bloc']!.family, 'state');
    });

    test('platform packs resolve before shared packs; other sees shared packs only', () {
      final catalog = Catalog.load(root.path);

      expect(catalog.packs.keys, ['tracker']);
      expect(catalog.knownOptions(), {
        'flutter': {'state': {'bloc'}, 'tracker': {'none'}},
        'other': {'tracker': {'none'}},
      });
      expect(catalog.packsFor('flutter')['state']!['bloc']!.fromProject, isFalse);
      expect(catalog.packsFor('other').keys, ['tracker']);
    });

    test('a project pack replaces the platform pack of the same family and option', () {
      writeProject('.ai/packs/state/bloc/pack.yaml', 'description: team bloc\n');

      final catalog = Catalog.load(root.path).withProjectPacks(project.path);

      expect(catalog.packsFor('flutter')['state']!['bloc']!.fromProject, isTrue);
    });

    test('a platform fragment at the same path replaces the core fragment; metas override core', () {
      final catalog = Catalog.load(root.path);

      final sources = catalog.fragmentsFor('flutter').map((f) => f.source).toList();

      expect(sources, [
        p.join('core', 'rules', 'code.md'),
        p.join('platforms', 'flutter', 'rules', 'lifecycle.md'),
        p.join('platforms', 'flutter', 'rules', 'testing.md'),
        p.join('platforms', 'flutter', 'skills', 'perf', 'perf.md'),
      ]);
      expect(catalog.fragmentsFor('other').map((f) => f.source), [
        p.join('core', 'rules', 'code.md'),
        p.join('core', 'rules', 'testing.md'),
      ]);
      expect(catalog.skillsFor('flutter')['docs']!.description, 'Dartdoc details');
      expect(catalog.skillsFor('other')['docs']!.description, 'generic docs');
      expect(catalog.skillsFor('other').containsKey('perf'), isFalse);
    });

    test('a project platform loads from .ai/platforms like a built-in one', () {
      writeProject('.ai/platforms/go/platform.yaml', '''
detect: [go.mod]
commands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }
''');
      writeProject('.ai/platforms/go/rules/errors.md', '---\ntarget: agents_md\norder: 71\n---\nWrap errors.\n');
      writeProject('.ai/platforms/go/packs/router/chi/pack.yaml', 'description: chi\n');

      final catalog = Catalog.load(root.path).withProjectPacks(project.path);
      final go = catalog.platforms['go']!;

      expect(go.fromProject, isTrue);
      expect(go.commands['format'], isNull);
      expect(go.docComment, isNull);
      expect(go.mcpServers, isEmpty);
      expect(go.fragments.single.source, p.join('.ai', 'platforms', 'go', 'rules', 'errors.md'));
      expect(catalog.knownOptions()['go'], {'router': {'chi'}, 'tracker': {'none'}});
      expect(catalog.fragmentsFor('go').map((f) => f.body.trim()), ['Generic code.', 'Generic testing.', 'Wrap errors.']);
    });

    test('a project platform with a built-in name replaces it whole', () {
      writeProject('.ai/platforms/flutter/platform.yaml', flutterYaml.replaceFirst('Flutter apps', 'Team Flutter'));

      final catalog = Catalog.load(root.path).withProjectPacks(project.path);

      expect(catalog.platforms['flutter']!.description, 'Team Flutter');
      expect(catalog.platforms['flutter']!.fragments, isEmpty);
      expect(catalog.knownOptions()['flutter'], {'tracker': {'none'}});
    });

    final broken = {
      'detect: [go.mod]\n': 'commands',
      'commands: { analyze: a, test: b, test_scoped: c, deps: d }\n': 'detect',
      'detect: [go.mod]\ncommands: { analyze: a, test: b, deps: d }\n': 'test_scoped',
      'detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nlinks: x\n': 'links',
      'detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nmcp_servers: { x: { args: [a] } }\n': 'mcp_servers.x',
    };
    broken.forEach((yaml, word) {
      test('invalid platform.yaml fails with its path ($word)', () {
        writeProject('.ai/platforms/go/platform.yaml', yaml);

        expect(
          () => Catalog.load(root.path).withProjectPacks(project.path),
          throwsA(isA<CatalogException>()
              .having((e) => e.source, 'source', p.join('.ai', 'platforms', 'go', 'platform.yaml'))
              .having((e) => e.message, 'message', contains(word))),
        );
      });
    });

    test('platform folder without platform.yaml fails', () {
      write('platforms/go/rules/a.md', '---\ntarget: agents_md\n---\nx\n');

      expect(
        () => Catalog.load(root.path),
        throwsA(isA<CatalogException>().having((e) => e.source, 'source', p.join('platforms', 'go', 'platform.yaml'))),
      );
    });

    test('platform named other is reserved', () {
      writeProject('.ai/platforms/other/platform.yaml', 'detect: [x]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\n');

      expect(
        () => Catalog.load(root.path).withProjectPacks(project.path),
        throwsA(isA<CatalogException>().having((e) => e.message, 'message', contains('reserved'))),
      );
    });

    test('platformFor builds other from the profile platform_config', () {
      final catalog = Catalog.load(root.path);
      final profile = Profile.parse('''
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make test-one, deps: make deps } }
''', knownOptions: catalog.knownOptions());

      final platform = catalog.platformFor(profile);

      expect(platform.name, 'other');
      expect(platform.detect, ['Makefile']);
      expect(platform.toContext(), {
        'name': 'other',
        'description': null,
        'detect': ['Makefile'],
        'commands': {'analyze': 'make lint', 'test': 'make test', 'test_scoped': 'make test-one', 'deps': 'make deps', 'format': null},
        'link': null,
        'doc_comment': null,
      });
      expect(platform.fragments, isEmpty);
    });
  });
}
