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
    expect(catalog.knownOptions(), {'state': {'bloc', 'riverpod'}});
  });

  test('pack folder without pack.yaml fails', () {
    write('packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nx\n');

    expect(() => Catalog.load(root.path), throwsA(isA<CatalogException>()));
  });
}
