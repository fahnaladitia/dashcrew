import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');
  late Directory project;

  Map<String, String> render(String yaml) {
    final profile = Profile.parse(yaml, knownOptions: catalog.knownOptions());
    final renderer = Renderer(catalog);
    return AntigravityAdapter(catalog, projectDir: project.path)
        .files(profile, renderer.render(profile), renderer.contextFor(profile), lockedFiles: const {});
  }

  setUp(() => project = Directory.systemTemp.createTempSync('dashcrew_antigravity_'));
  tearDown(() => project.deleteSync(recursive: true));

  test('skills match the codex files byte for byte', () {
    const yaml = 'toolkit: 1\nagents: [antigravity]\nstack: { state: bloc }\n';
    final profile = Profile.parse(yaml, knownOptions: catalog.knownOptions());
    final renderer = Renderer(catalog);
    final codex = CodexAdapter(catalog, projectDir: project.path)
        .files(profile, renderer.render(profile), renderer.contextFor(profile), lockedFiles: const {});

    final antigravity = render(yaml);

    for (final path in antigravity.keys.where((path) => path.startsWith('.agents/skills/'))) {
      expect(antigravity[path], codex[path], reason: path);
    }
    expect(antigravity.keys, contains('.agents/skills/bloc-patterns/SKILL.md'));
  });

  test('roles land in .agents/agents with a tools allowlist from their capability', () {
    final files = render('toolkit: 1\nagents: [antigravity]\n');

    final reviewer = files['.agents/agents/flutter-code-reviewer.md']!;
    expect(reviewer, startsWith('---\nname: flutter-code-reviewer\ndescription: "'));
    expect(reviewer, contains('\nmodel: inherit\ntools: [view_file, list_dir, find_by_name, grep_search, run_command]\n---\n\n'));
    expect(files['.agents/agents/flutter-planner.md'], contains('tools: [view_file, list_dir, find_by_name, grep_search, run_command, write_to_file]'));
    expect(
      files['.agents/agents/flutter-engineer.md'],
      contains('tools: [view_file, list_dir, find_by_name, grep_search, run_command, write_to_file, replace_file_content, multi_replace_file_content]'),
    );
    expect(files.keys, isNot(contains('.agents/agents/design-refiner.md')));
  });

  test('every allowlisted tool is a documented antigravity tool name', () {
    for (final tools in AntigravityAdapter.toolsByCapability.values) {
      expect(AntigravityAdapter.knownTools, containsAll(tools));
    }
    expect(AntigravityAdapter.toolsByCapability.keys, RoleMeta.capabilities);
  });
  group('guard hook in .agents/hooks.json', () {
    File hooks() => File('${project.path}/.agents/hooks.json');
    List<String> merge() => AntigravityAdapter(catalog, projectDir: project.path).merge(const {}, <String>[]).written;

    test('is kept under its own key next to other hooks, and written once', () {
      hooks()
        ..parent.createSync(recursive: true)
        ..writeAsStringSync(jsonEncode({
          'my-linter-hook': {
            'PostToolUse': [
              {'matcher': '', 'hooks': [{'type': 'command', 'command': './lint.sh'}]},
            ],
          },
        }));

      expect(merge(), contains('.agents/hooks.json'));
      expect(merge(), isNot(contains('.agents/hooks.json')));

      final config = jsonDecode(hooks().readAsStringSync()) as Map<String, Object?>;
      expect(config.keys, ['my-linter-hook', 'dashcrew-guard']);
      expect(config['dashcrew-guard'], AntigravityAdapter.guardHook);
    });
  });
}
