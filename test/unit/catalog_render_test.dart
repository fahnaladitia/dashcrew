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
