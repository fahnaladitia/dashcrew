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

  group('skill index', () {
    String index(Catalog c, String yaml) => Renderer(c).render(profile(yaml, c)).firstWhere((t) => t.target == 'agents_md').content;

    const indexBody = '{{#skill_index}}- {{name}}{{#review}} (review){{/review}}: {{when_to_use}}\n{{/skill_index}}';

    test('lists every rendered skill once, sorted by name, from core and selected packs', () {
      final c = Catalog(
        core: [
          fragment('core/rules/index.md', indexBody),
          fragment('core/skills/zeta/a.md', 'Z', target: 'skill/zeta'),
          fragment('core/skills/zeta/b.md', 'Z2', target: 'skill/zeta'),
          fragment('core/skills/alpha/a.md', 'A', target: 'skill/alpha'),
          fragment('core/workflows/plan/a.md', 'P', target: 'workflow/plan'),
        ],
        packs: {
          'state': {
            'bloc': Pack(
              family: 'state',
              option: 'bloc',
              params: const {},
              fragments: [fragment('packs/state/bloc/s.md', 'B', target: 'skill/bloc-patterns')],
              skills: const {'bloc-patterns': DocMeta(description: 'Bloc patterns.', whenToUse: 'On a Bloc.')},
            ),
          },
        },
        skills: const {
          'zeta': DocMeta(description: 'Zeta.', whenToUse: 'Last.', review: true),
          'alpha': DocMeta(description: 'Alpha skill.'),
        },
        workflows: const {'plan': DocMeta(description: 'Plan.')},
      );

      final content = index(c, 'toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n');

      expect(content, '- alpha: Alpha skill.\n- bloc-patterns: On a Bloc.\n- zeta (review): Last.\n');
    });

    test('a skill whose fragments are all filtered out, or that has no meta, is not listed', () {
      final c = Catalog(
        core: [
          fragment('core/rules/index.md', indexBody),
          fragment('core/skills/gated/a.md', 'G', target: 'skill/gated', when: {'policy.formatter': true}),
          fragment('core/skills/orphan/a.md', 'O', target: 'skill/orphan'),
          fragment('core/skills/kept/a.md', 'K', target: 'skill/kept'),
        ],
        packs: const {},
        skills: const {
          'gated': DocMeta(description: 'Gated.'),
          'kept': DocMeta(description: 'Kept.', whenToUse: 'Always.'),
        },
      );

      expect(index(c, 'toolkit: 1\nagents: [claude-code]\n'), '- kept: Always.\n');
    });
  });

  group('platform', () {
    const commands = {'analyze': 'lint', 'test': 'test all', 'test_scoped': 'test one', 'deps': 'fetch', 'format': null};
    Catalog withPlatform() => Catalog(
          core: [
            fragment('core/rules/testing.md', 'Run `{{platform.commands.test_scoped}}`{{#platform.commands.format}}, then `{{.}}`{{/platform.commands.format}}.', order: 10),
            fragment('core/rules/code.md', 'Generic code.', order: 20),
          ],
          packs: {
            'tracker': {'none': Pack(family: 'tracker', option: 'none', params: const {}, fragments: [fragment('packs/tracker/none/a.md', 'No tracker.', order: 90)])},
          },
          platforms: {
            'flutter': PlatformSpec(
              name: 'flutter',
              root: 'platforms/flutter',
              detect: const ['pubspec.yaml'],
              commands: {...commands, 'format': 'fmt'},
              fragments: [
                fragment('platforms/flutter/rules/code.md', 'Platform code.', order: 20),
                fragment('platforms/flutter/rules/lifecycle.md', 'Dispose.', order: 15),
              ],
              packs: {
                'state': {'bloc': Pack(family: 'state', option: 'bloc', params: const {}, fragments: [fragment('platforms/flutter/packs/state/bloc/a.md', 'Bloc.', order: 80)])},
              },
            ),
          },
        );

    test('the selected platform adds its fragments, replaces core ones at the same path and fills platform.*', () {
      final c = withPlatform();

      final targets = Renderer(c).render(profile('toolkit: 1\nagents: [claude-code]\nstack: { state: bloc, tracker: none }\n', c));

      expect(targets.single.content, 'Run `test one`, then `fmt`.\n\nDispose.\n\nPlatform code.\n\nBloc.\n\nNo tracker.\n');
    });

    test('other renders core only, with the commands of platform_config', () {
      final c = withPlatform();
      final yaml = '''
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: make deps } }
stack: { tracker: none }
''';

      final targets = Renderer(c).render(profile(yaml, c));

      expect(targets.single.content, 'Run `make one`.\n\nGeneric code.\n\nNo tracker.\n');
      expect(Renderer(c).contextFor(profile(yaml, c))['platform'], containsPair('name', 'other'));
    });
  });
}
