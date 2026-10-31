import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');
  late Directory project;

  Map<String, String> render(String yaml) {
    final profile = Profile.parse(yaml, knownOptions: catalog.knownOptions());
    final renderer = Renderer(catalog);
    return CodexAdapter(catalog, projectDir: project.path)
        .files(profile, renderer.render(profile), renderer.contextFor(profile), lockedFiles: const {});
  }

  const fullStack = '''
toolkit: 1
agents: [codex]
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
''';

  setUp(() => project = Directory.systemTemp.createTempSync('dashcrew_codex_'));
  tearDown(() => project.deleteSync(recursive: true));

  test('skills and workflows land in .agents/skills with name and description only', () {
    final files = render(fullStack);

    final plan = files['.agents/skills/plan/SKILL.md']!;
    final frontMatter = plan.substring(4, plan.indexOf('\n---\n', 4));
    expect(frontMatter.split('\n').map((line) => line.split(':').first), ['name', 'description']);
    expect(plan, startsWith('---\nname: plan\ndescription: "Plan a change'));
    expect(plan, contains('\n---\n\nArguments: `<task description or confirmed step 0 prompt>`\n\n# /plan'));
    expect(files.keys, contains('.agents/skills/bloc-patterns/SKILL.md'));
    expect(files.keys.where((path) => !path.startsWith('.agents/skills/')), isEmpty);
  });

  test('when_to_use is folded into the description', () {
    final skill = render(fullStack)['.agents/skills/data-layer-tdd/SKILL.md']!;

    expect(RegExp(r'^description: ".+ Use when: .+"$', multiLine: true).hasMatch(skill), isTrue);
  });

  test('bodies match the claude code skills', () {
    final profile = Profile.parse(fullStack.replaceFirst('[codex]', '[claude-code]'), knownOptions: catalog.knownOptions());
    final renderer = Renderer(catalog);
    final claude = ClaudeCodeAdapter(catalog, projectDir: project.path)
        .files(profile, renderer.render(profile), renderer.contextFor(profile), lockedFiles: const {});
    String body(String skill) => skill.substring(skill.indexOf('\n---\n', 4) + 5);

    final codex = render(fullStack);
    for (final path in codex.keys) {
      final name = path.split('/')[2];
      expect(body(codex[path]!), endsWith(body(claude['.claude/skills/$name/SKILL.md']!)), reason: name);
    }
  });

  test('the skill list of the full sample stack fits the codex budget', () {
    final files = render(fullStack);

    expect(CodexAdapter.skillListChars(files.values), lessThan(CodexAdapter.skillListBudget));
  });

  test('skill list size counts name and description of each skill', () {
    expect(CodexAdapter.skillListChars(['---\nname: ab\ndescription: "cde"\n---\n\nbody\n']), 5);
  });
}
