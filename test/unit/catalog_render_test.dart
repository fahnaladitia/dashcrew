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
