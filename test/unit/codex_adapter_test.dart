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
    expect(plan, contains('\n---\n\nArguments: `<task description or confirmed step 0 prompt>`\n\n# plan'));
    expect(files.keys, contains('.agents/skills/bloc-patterns/SKILL.md'));
    expect(files.keys.where((path) => !path.startsWith('.agents/skills/') && !path.startsWith('.codex/agents/')), isEmpty);
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
    for (final path in codex.keys.where((path) => path.startsWith('.agents/skills/'))) {
      final name = path.split('/')[2];
      expect(body(codex[path]!), endsWith(body(claude['.claude/skills/$name/SKILL.md']!)), reason: name);
    }
  });

  test('the skill list of the full sample stack fits the codex budget', () {
    final files = render(fullStack);

    final skills = files.entries.where((e) => e.key.startsWith('.agents/skills/')).map((e) => e.value);

    expect(CodexAdapter.skillListChars(skills), lessThan(CodexAdapter.skillListBudget));
  });

  test('skill list size counts name and description of each skill', () {
    expect(CodexAdapter.skillListChars(['---\nname: ab\ndescription: "cde"\n---\n\nbody\n']), 5);
  });

  group('guard hook in .codex/hooks.json', () {
    String hooksPath() => '${project.path}/.codex/hooks.json';
    ({List<String> written, Set<String> ownedMcp}) merge() =>
        CodexAdapter(catalog, projectDir: project.path).merge(const {}, <String>[]);

    test('is added next to the user hooks and written once', () {
      File(hooksPath())
        ..parent.createSync(recursive: true)
        ..writeAsStringSync(jsonEncode({
          'hooks': {
            'PreToolUse': [
              {'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': 'other-hook'}]},
            ],
            'PostToolUse': [
              {'hooks': [{'type': 'command', 'command': 'notify'}]},
            ],
          },
        }));

      expect(merge().written, contains('.codex/hooks.json'));
      expect(merge().written, isNot(contains('.codex/hooks.json')));

      final hooks = jsonDecode(File(hooksPath()).readAsStringSync()) as Map<String, Object?>;
      final preToolUse = ((hooks['hooks'] as Map)['PreToolUse'] as List).cast<Map>();
      expect(preToolUse, hasLength(2));
      expect(jsonEncode(preToolUse.last), contains('.ai/bin/guard'));
      expect(preToolUse.last['matcher'], CodexAdapter.guardMatcher);
      expect(jsonEncode(hooks), contains('notify'));
    });

    test('an edited dashcrew entry is replaced, not duplicated', () {
      merge();
      final text = File(hooksPath()).readAsStringSync().replaceFirst('"timeout": 30', '"timeout": 5');
      File(hooksPath()).writeAsStringSync(text);

      merge();

      final hooks = File(hooksPath()).readAsStringSync();
      expect('.ai/bin/guard'.allMatches(hooks), hasLength(1));
      expect(hooks, contains('"timeout": 30'));
    });
  });

  group('mcp block in .codex/config.toml', () {
    File config() => File('${project.path}/.codex/config.toml');
    const block = '''
# dashcrew:begin - written by dashcrew sync; edit outside this block
[mcp_servers.dart-mcp-server]
command = "dart"
args = ["mcp-server"]
# dashcrew:end
''';
    ({List<String> written, Set<String> ownedMcp}) merge(List<String> warnings) =>
        CodexAdapter(catalog, projectDir: project.path).merge(const {}, warnings);

    test('a new config holds the block only', () {
      final result = merge([]);

      expect(config().readAsStringSync(), block);
      expect(result.written, contains('.codex/config.toml'));
      expect(result.ownedMcp, {'dart-mcp-server'});
    });

    test('the block goes after the user settings, which stay byte for byte', () {
      const user = 'model = "some-model"\n\n[mcp_servers.graph]\ncommand = "graph"\n';
      config()
        ..parent.createSync(recursive: true)
        ..writeAsStringSync(user);

      merge([]);

      expect(config().readAsStringSync(), '$user\n$block');
      expect(merge([]).written, isNot(contains('.codex/config.toml')));
    });

    test('a stale block is replaced in place and text after it is kept', () {
      config()
        ..parent.createSync(recursive: true)
        ..writeAsStringSync('model = "m"\n\n# dashcrew:begin\n[mcp_servers.old]\ncommand = "x"\n# dashcrew:end\n\n[profiles.fast]\nmodel = "f"\n');

      merge([]);

      expect(config().readAsStringSync(), 'model = "m"\n\n$block\n[profiles.fast]\nmodel = "f"\n');
    });

    test('a server the user defines outside the block is left alone with a warning', () {
      config()
        ..parent.createSync(recursive: true)
        ..writeAsStringSync('[mcp_servers."dart-mcp-server"]\ncommand = "fvm"\n');
      final warnings = <String>[];

      final result = merge(warnings);

      expect(config().readAsStringSync(), '[mcp_servers."dart-mcp-server"]\ncommand = "fvm"\n');
      expect(result.ownedMcp, isEmpty);
      expect(warnings.single, contains('dart-mcp-server'));
    });
  });
  group('roles in .codex/agents', () {
    test('one toml file per generated role, with effort and the role body', () {
      final files = render(fullStack);

      final reviewer = files['.codex/agents/reviewer.toml']!;
      expect(reviewer, startsWith('name = "reviewer"\ndescription = "'));
      expect(reviewer, contains('\nmodel_reasoning_effort = "high"\n'));
      expect(reviewer, contains('\ndeveloper_instructions = """\n'));
      expect(reviewer, endsWith('"""\n'));
      expect(files['.codex/agents/explorer.toml'], contains('\nmodel_reasoning_effort = "medium"\n'));
    });

    test('a role gated by an unselected pack is not generated', () {
      final gated = Catalog(
        core: const [
          Fragment(source: 'core/roles/gated/role.md', target: 'role/gated', order: 100, rules: [], when: {}, body: 'Gated.'),
          Fragment(source: 'core/roles/open/role.md', target: 'role/open', order: 100, rules: [], when: {}, body: 'Open.'),
        ],
        packs: const {},
        roles: const {
          'gated': RoleMeta(name: 'gated', description: 'Gated.', capability: 'read', when: {'stack.design.use': ['figma']}),
          'open': RoleMeta(name: 'open', description: 'Open.', capability: 'read'),
        },
      );
      final profile = Profile.parse('toolkit: 1\nagents: [codex]\n', knownOptions: gated.knownOptions());
      final renderer = Renderer(gated);

      final files = CodexAdapter(gated, projectDir: project.path)
          .files(profile, renderer.render(profile), renderer.contextFor(profile), lockedFiles: const {});

      expect(files.keys.where((path) => path.startsWith('.codex/agents/')), ['.codex/agents/open.toml']);
    });

    test('backslashes and triple quotes in a body are escaped', () {
      expect(CodexAdapter.tomlMultiline(r'a \d """ b'), '"""\n' r'a \\d ""\" b' '"""');
    });
  });
}
