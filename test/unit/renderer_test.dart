import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

Fragment fragment(
  String source,
  String body, {
  String target = 'agents_md',
  int order = 100,
  List<String> rules = const [],
  Map<String, Object?> when = const {},
}) =>
    Fragment(source: source, target: target, order: order, rules: rules, when: when, body: body);

Catalog catalog({List<Fragment> core = const [], Map<String, Map<String, Pack>> packs = const {}}) =>
    Catalog(core: core, packs: packs);

Profile profile(String yaml, Catalog catalog) => Profile.parse(yaml, knownOptions: catalog.knownOptions());

void main() {
  test('core fragments render and join in order', () {
    final c = catalog(core: [
      fragment('core/b.md', 'Second', order: 20),
      fragment('core/a.md', 'First', order: 10),
    ]);

    final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\n', c));

    expect(targets.single.target, 'agents_md');
    expect(targets.single.content, 'First\n\nSecond\n');
  });

  test('same order falls back to source path', () {
    final c = catalog(core: [fragment('core/z.md', 'Z'), fragment('core/a.md', 'A')]);

    final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\n', c));

    expect(targets.single.content, 'A\n\nZ\n');
  });

  test('only selected packs are rendered, with params merged over defaults', () {
    final c = catalog(packs: {
      'state': {
        'bloc': Pack(
          family: 'state',
          option: 'bloc',
          params: {'reference_folder': 'lib/default'},
          fragments: [fragment('packs/state/bloc/r.md', 'Bloc ref {{pack.state.reference_folder}}')],
        ),
        'riverpod': Pack(
          family: 'state',
          option: 'riverpod',
          params: const {},
          fragments: [fragment('packs/state/riverpod/r.md', 'Riverpod')],
        ),
      },
    });

    final targets = Renderer(c).render(profile('''
toolkit: 1
agents: [claude-code]
stack: { state: { use: bloc, reference_folder: lib/ui/orders/blocs } }
''', c));

    expect(targets.single.content, 'Bloc ref lib/ui/orders/blocs\n');
  });

  test('when filters fragments against the profile', () {
    final c = catalog(core: [
      fragment('core/commit.md', 'Never commit.', when: {'policy.agent_commit': false}),
      fragment('core/commit_ok.md', 'Commits allowed.', when: {'policy.agent_commit': true}),
      fragment('core/lang.md', 'Indonesian or Malay docs.', when: {'language.dartdoc': ['id', 'ms']}),
    ]);

    final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\nlanguage: { dartdoc: id }\n', c));

    expect(targets.single.content, 'Never commit.\n\nIndonesian or Malay docs.\n');
  });

  test('profile values are substituted without html escaping', () {
    final c = catalog(core: [fragment('core/doc.md', 'Max {{policy.dartdoc_max_lines}} lines in {{paths.plans}} <b>&</b>')]);

    final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\n', c));

    expect(targets.single.content, 'Max 3 lines in .ai/plans <b>&</b>\n');
  });

  test('missing variable fails with the fragment source', () {
    final c = catalog(core: [fragment('core/typo.md', '{{policy.dartdoc_max_line}}')]);

    expect(
      () => Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\n', c)),
      throwsA(isA<RenderException>().having((e) => e.source, 'source', 'core/typo.md')),
    );
  });

  test('targets are separated and rule ids collected per target', () {
    final c = catalog(core: [
      fragment('core/git.md', 'Git', rules: ['git.no-agent-commit']),
      fragment('core/plan.md', 'Plan step', target: 'workflow/plan', rules: ['plan.verdict']),
      fragment('core/plan2.md', 'Plan gate', target: 'workflow/plan', rules: ['plan.wait', 'plan.verdict']),
    ]);

    final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\n', c));
    final byTarget = {for (final t in targets) t.target: t};

    expect(byTarget.keys, ['agents_md', 'workflow/plan']);
    expect(byTarget['agents_md']!.rules, {'git.no-agent-commit'});
    expect(byTarget['workflow/plan']!.rules, {'plan.verdict', 'plan.wait'});
  });
}
