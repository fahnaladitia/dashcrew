import 'dart:convert';

import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');

  List<RenderedTarget> render(String yaml) =>
      Renderer(catalog).render(Profile.parse(yaml, knownOptions: catalog.knownOptions()));

  int agentsMdBytes(List<RenderedTarget> targets) =>
      utf8.encode(targets.singleWhere((t) => t.target == 'agents_md').content).length;

  test('shipped catalog renders with no stack selected', () {
    final targets = render('toolkit: 1\nagents: [claude-code]\n');

    expect(targets.map((t) => t.target), contains('agents_md'));
    expect(agentsMdBytes(targets), lessThan(32 * 1024));
  });

  test('shipped catalog renders with every non-none option of each family', () {
    final families = catalog.knownOptions();
    for (final family in families.keys) {
      for (final option in families[family]!.where((o) => o != 'none')) {
        final targets = render('toolkit: 1\nagents: [claude-code]\nstack: { $family: $option }\n');

        expect(agentsMdBytes(targets), lessThan(32 * 1024), reason: '$family/$option');
      }
    }
  });

  test('every family set to other renders one line per family and stays under the size limit', () {
    final families = catalog.knownOptions().keys.toList()..sort();
    final stack = [for (final family in families) '  $family: { use: other, name: lib_$family }'].join('\n');
    final targets = render('toolkit: 1\nagents: [claude-code]\nstack:\n$stack\n');
    final agentsMd = targets.singleWhere((t) => t.target == 'agents_md');

    expect(agentsMd.content, contains('## Libraries without a dashcrew pack'));
    for (final family in families) {
      expect(agentsMd.content, contains('- $family: `lib_$family`'));
    }
    expect(agentsMd.rules, contains('stack.other-follow-official-docs'));
    expect(agentsMdBytes(targets), lessThan(32 * 1024));
  });

  test('no other choice renders no libraries section', () {
    final targets = render('toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n');

    expect(targets.singleWhere((t) => t.target == 'agents_md').content, isNot(contains('without a dashcrew pack')));
  });

  test('flavors render one run line each, in name order, with the build wrapper', () {
    final targets = render('''
toolkit: 1
agents: [claude-code]
project:
  flavors: { staging: lib/main_staging.dart, dev: lib/main_dev.dart }
  build_wrapper: tool/run.sh
''');
    final agentsMd = targets.singleWhere((t) => t.target == 'agents_md').content;

    expect(
      agentsMd,
      contains('## Build and run\n\n'
          '- `dev`: `flutter run --flavor dev -t lib/main_dev.dart`\n'
          '- `staging`: `flutter run --flavor staging -t lib/main_staging.dart`\n'
          '- Real builds go through `tool/run.sh`'),
    );
  });

  test('no flavors renders no build section', () {
    final targets = render('toolkit: 1\nagents: [claude-code]\nproject: { build_wrapper: tool/run.sh }\n');

    expect(targets.singleWhere((t) => t.target == 'agents_md').content, isNot(contains('## Build and run')));
  });

  group('state packs', () {
    String agentsMd(String stack) =>
        render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n').singleWhere((t) => t.target == 'agents_md').content;
    Set<String> targetsOf(String stack) => render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n').map((t) => t.target).toSet();

    test('riverpod renders its rules and patterns skill, hand-written providers by default', () {
      final rules = agentsMd('state: riverpod');

      expect(rules, contains('## State: Riverpod'));
      expect(rules, contains('`ref.mounted`'));
      expect(rules, contains('declared by hand'));
      expect(rules, isNot(contains('@riverpod')));
      expect(targetsOf('state: riverpod'), containsAll(['skill/riverpod-patterns', 'workflow/tdd']));
    });

    test('riverpod with codegen asks for generated providers', () {
      final rules = agentsMd('state: { use: riverpod, codegen: true }');

      expect(rules, contains('`@riverpod`'));
      expect(rules, isNot(contains('declared by hand')));
    });

    test('provider renders its rules and patterns skill', () {
      expect(agentsMd('state: provider'), contains('## State: provider'));
      expect(targetsOf('state: provider'), contains('skill/provider-patterns'));
    });

    test('none keeps state in Flutter and adds no skill', () {
      expect(agentsMd('state: none'), contains('## State: Flutter only'));
      expect(targetsOf('state: none').where((t) => t.endsWith('-patterns')), isEmpty);
    });
  });

  final headings = {
    'di: riverpod': '## Dependency injection: Riverpod',
    'di: provider': '## Dependency injection: provider',
    'di: none': '## Dependency injection: constructor parameters',
    'router: go_router': '## Navigation: go_router',
    'router: auto_route': '## Navigation: auto_route',
    'router: none': '## Navigation: Navigator',
    'network: http': '## Network: http',
    'network: retrofit': '## Network: Retrofit',
    'network: none': '## Network: no HTTP package',
    'storage: shared_preferences': '## Local storage: shared_preferences',
    'storage: sqflite': '## Local storage: sqflite',
    'test: mocktail': '## Tests: mocktail',
    'test: mockito': '## Tests: mockito',
    'test: none': '## Tests: no mocking convention',
    'analytics: firebase': '## Analytics: Firebase',
    'analytics: mixpanel': '## Analytics: Mixpanel',
  };
  headings.forEach((stack, heading) {
    test('$stack renders "$heading"', () {
      final agentsMd = render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n').singleWhere((t) => t.target == 'agents_md');

      expect(agentsMd.content, contains(heading));
    });
  });

  test('get_it with injectable switches registration to annotations', () {
    String rules(String di) =>
        render('toolkit: 1\nagents: [claude-code]\nstack: { di: $di }\n').singleWhere((t) => t.target == 'agents_md').content;

    expect(rules('{ use: get_it, injectable: true }'), contains('`@lazySingleton`'));
    expect(rules('{ use: get_it, injectable: true }'), isNot(contains('Register every new dependency')));
    expect(rules('get_it'), contains('Register every new dependency'));
  });

  test('di riverpod with a different state library says how state holders get dependencies', () {
    String rules(String stack) =>
        render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n').singleWhere((t) => t.target == 'agents_md').content;

    expect(rules('di: riverpod, state: bloc'), contains('`ConsumerWidget`'));
    expect(rules('di: riverpod, state: riverpod'), isNot(contains('State is not Riverpod')));
  });

  test('mocktail with bloc adds blocTest to the tdd workflow; manual fakes do not', () {
    String tdd(String stack) =>
        render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n').singleWhere((t) => t.target == 'workflow/tdd').content;

    expect(tdd('state: bloc, test: mocktail'), contains('blocTest'));
    expect(tdd('state: bloc, test: manual-fakes'), isNot(contains('blocTest')));
  });

  test('the largest option of every family together stays under the AGENTS.md size limit', () {
    final families = catalog.knownOptions();
    final largest = <String, String>{};
    for (final family in families.keys) {
      var bytes = -1;
      for (final option in families[family]!) {
        final size = agentsMdBytes(render('toolkit: 1\nagents: [claude-code]\nstack: { $family: $option }\n'));
        if (size > bytes) {
          bytes = size;
          largest[family] = option;
        }
      }
    }
    final stack = largest.entries.map((e) => '${e.key}: ${e.value}').join(', ');

    final targets = render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n');

    expect(agentsMdBytes(targets), lessThan(Doctor.agentsMdLimitFor({'antigravity'})), reason: stack);
  });

  group('pr template', () {
    List<String> headings(String stack) => render('toolkit: 1\nagents: [claude-code]\nstack: { $stack }\n')
        .singleWhere((t) => t.target == 'pr_template')
        .content
        .split('\n')
        .where((line) => line.startsWith('## '))
        .toList();

    test('core sections only, notes last', () {
      expect(headings(''), ['## Summary', '## Changes', '## Affected files', '## Testing', '## Notes / not done']);
    });

    test('qa and analytics packs add their sections before notes', () {
      for (final analytics in ['posthog', 'firebase', 'mixpanel']) {
        expect(
          headings('qa: semantics-identifier, analytics: $analytics'),
          ['## Summary', '## Changes', '## Affected files', '## Testing', '## QA Identifiers', '## Analytics', '## Notes / not done'],
          reason: analytics,
        );
      }
    });
  });

  test('sample full stack stays under the AGENTS.md size limit with every policy flipped', () {
    final targets = render('''
toolkit: 1
agents: [claude-code, codex]
language: { dartdoc: id, pr: id, chat: id }
stack:
  state: bloc
  di: get_it
  router: getx
  network: dio
  storage: hive
  test: manual-fakes
  analytics: posthog
  tracker: clickup
  qa: semantics-identifier
  design: figma
project:
  local_packages: [core, shared]
  base_branches: [develop, main]
  build_wrapper: tool/run.sh
policy:
  agent_commit: true
  formatter: true
  tier_gates: false
  terse_prompts: true
''');

    expect(agentsMdBytes(targets), lessThan(32 * 1024));
    expect(targets.map((t) => t.target), containsAll(['skill/bloc-patterns', 'role/flutter-engineer', 'workflow/plan']));
  });
}
