import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:dashcrew/src/package_root.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  late Directory project;
  final catalog = Catalog.load('.');

  String read(String relative) => File(p.join(project.path, relative)).readAsStringSync();
  bool exists(String relative) => File(p.join(project.path, relative)).existsSync();
  void write(String relative, String content) {
    final file = File(p.join(project.path, relative));
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
  }

  SyncResult sync({bool force = false, bool adopt = false}) =>
      Sync(catalog: catalog, projectDir: project.path, compileGuard: false).run(force: force, adopt: adopt);

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_sync_');
    write('.ai/project.yaml', '''
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  di: get_it
  analytics: posthog
project: { local_packages: [core] }
''');
  });
  tearDown(() => project.deleteSync(recursive: true));

  test('first sync writes the claude code toolkit', () {
    final result = sync();

    expect(result.written, isNotEmpty);
    expect(read('AGENTS.md'), contains('dashcrew:generated'));
    expect(read('AGENTS.md'), contains('<!-- dashcrew:project:begin -->'));
    expect(read('CLAUDE.md').trim(), '@AGENTS.md');

    final planner = read('.claude/agents/flutter-planner.md');
    expect(planner, startsWith('---\nname: flutter-planner\n'));
    expect(planner, contains('tools: Read, Write, Grep, Glob, Bash'));
    expect(planner, contains('model: opus'));
    expect(read('.claude/agents/flutter-code-reviewer.md'), contains('tools: Read, Grep, Glob, Bash\n'));

    expect(read('.claude/skills/plan/SKILL.md'), startsWith('---\nname: plan\ndescription: '));
    expect(read('.claude/skills/bloc-patterns/SKILL.md'), contains('name: bloc-patterns'));

    final guard = jsonDecode(read('.ai/guard.json')) as Map<String, Object?>;
    expect(guard['forbidden_imports'], ['package:posthog_flutter']);

    final settings = jsonDecode(read('.claude/settings.json')) as Map<String, Object?>;
    final preToolUse = ((settings['hooks'] as Map)['PreToolUse'] as List).cast<Map>();
    expect(preToolUse.where((h) => jsonEncode(h).contains('.ai/bin/guard')), hasLength(1));

    final mcp = jsonDecode(read('.mcp.json')) as Map<String, Object?>;
    expect((mcp['mcpServers'] as Map).keys, contains('dart-mcp-server'));

    expect(exists('.ai/bin/guard'), isTrue);
    expect(exists('.ai/lock.json'), isTrue);
  });

  test('second sync changes nothing', () {
    sync();
    final result = sync();

    expect(result.written, isEmpty);
    expect(result.removed, isEmpty);
  });

  test('foreign settings hooks and mcp servers are preserved', () {
    write('.claude/settings.json', jsonEncode({
      'hooks': {
        'PreToolUse': [
          {
            'matcher': 'Bash',
            'hooks': [
              {'type': 'command', 'command': '/usr/local/bin/other-hook'},
            ],
          },
        ],
        'Stop': [
          {
            'hooks': [
              {'type': 'command', 'command': 'notify'},
            ],
          },
        ],
      },
      'permissions': {'allow': ['Bash(ls)']},
    }));
    write('.mcp.json', jsonEncode({
      'mcpServers': {
        'graph': {'command': 'graph', 'args': ['serve']},
      },
    }));

    sync();
    sync();

    final settings = read('.claude/settings.json');
    expect(settings, contains('/usr/local/bin/other-hook'));
    expect(settings, contains('notify'));
    expect(settings, contains('Bash(ls)'));
    expect('.ai/bin/guard'.allMatches(settings), hasLength(1));
    expect(read('.mcp.json'), contains('"graph"'));
  });

  test('the lock records owned mcp servers per agent', () {
    sync();

    final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
    expect(lock['mcp_servers'], {
      'claude-code': ['dart-mcp-server'],
    });
  });

  test('a lock with the old mcp_servers list is read as claude code ownership', () {
    sync();
    final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
    lock['mcp_servers'] = ['dart-mcp-server'];
    write('.ai/lock.json', jsonEncode(lock));
    write('.mcp.json', jsonEncode({
      'mcpServers': {
        'dart-mcp-server': {'command': 'old'},
      },
    }));

    final result = sync();

    expect(result.warnings, isEmpty);
    expect(read('.mcp.json'), isNot(contains('"old"')));
    expect((jsonDecode(read('.ai/lock.json')) as Map)['mcp_servers'], {
      'claude-code': ['dart-mcp-server'],
    });
  });

  test('adapterFor names the supported agents and rejects the others', () {
    expect(adapterFor('claude-code', catalog, project.path).agent, 'claude-code');
    expect(adapterFor('codex', catalog, project.path).agent, 'codex');
    expect(adapterFor('antigravity', catalog, project.path).agent, 'antigravity');
    expect(() => adapterFor('cursor', catalog, project.path), throwsA(isA<SyncException>()));
  });

  test('hand-edited generated file blocks sync until --force', () {
    sync();
    write('.claude/skills/plan/SKILL.md', 'edited by hand\n');

    expect(() => sync(), throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('.claude/skills/plan/SKILL.md'))));

    sync(force: true);
    expect(read('.claude/skills/plan/SKILL.md'), isNot('edited by hand\n'));
  });

  test('project block in AGENTS.md survives sync', () {
    sync();
    final agents = read('AGENTS.md').replaceFirst(
      '<!-- dashcrew:project:begin -->',
      '<!-- dashcrew:project:begin -->\nTeam rule: deploy on Fridays never.',
    );
    write('AGENTS.md', agents);

    sync();

    expect(read('AGENTS.md'), contains('Team rule: deploy on Fridays never.'));
  });

  test('existing AGENTS.md not made by dashcrew needs --adopt', () {
    write('AGENTS.md', 'Our own rules.\n');

    expect(() => sync(), throwsA(isA<SyncException>()));

    sync(adopt: true);
    expect(read('AGENTS.md'), contains('dashcrew:generated'));
    expect(read('AGENTS.md'), contains('Our own rules.'));
  });

  test('existing CLAUDE.md is kept and reported when it does not import AGENTS.md', () {
    write('CLAUDE.md', 'Personal notes.\n');

    final result = sync();

    expect(read('CLAUDE.md'), 'Personal notes.\n');
    expect(result.warnings.join('\n'), contains('CLAUDE.md'));
  });

  test('role gated by an unselected pack is not generated', () {
    sync();

    expect(exists('.claude/agents/design-refiner.md'), isFalse);
    expect(exists('.claude/agents/flutter-engineer.md'), isTrue);
  });

  test('files no longer generated are removed', () {
    sync();
    expect(exists('.claude/skills/bloc-patterns/SKILL.md'), isTrue);

    write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\n');
    final result = sync();

    expect(exists('.claude/skills/bloc-patterns/SKILL.md'), isFalse);
    expect(result.removed, contains('.claude/skills/bloc-patterns/SKILL.md'));
  });

  test('codex alone writes its toolkit and nothing for claude code', () {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [codex]\nstack: { analytics: posthog }\n');

    sync();

    expect(exists('AGENTS.md'), isTrue);
    expect(exists('.agents/skills/plan/SKILL.md'), isTrue);
    expect(exists('.codex/agents/flutter-planner.toml'), isTrue);
    expect(read('.codex/hooks.json'), contains('.ai/bin/guard'));
    expect(read('.codex/config.toml'), contains('[mcp_servers.dart-mcp-server]'));
    expect(exists('CLAUDE.md'), isFalse);
    expect(exists('.claude/settings.json'), isFalse);
    final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
    expect(lock['mcp_servers'], {
      'codex': ['dart-mcp-server'],
    });
    expect(sync().written, isEmpty);
  });

  test('antigravity alone writes its toolkit and nothing for the other agents', () {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [antigravity]\n');

    sync();

    expect(exists('.agents/skills/plan/SKILL.md'), isTrue);
    expect(exists('.agents/agents/flutter-planner.md'), isTrue);
    expect(read('.agents/hooks.json'), contains('dashcrew-guard'));
    expect(read('.agents/mcp_config.json'), contains('dart-mcp-server'));
    expect(exists('CLAUDE.md'), isFalse);
    expect(exists('.codex/hooks.json'), isFalse);
    final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
    expect(lock['mcp_servers'], {
      'antigravity': ['dart-mcp-server'],
    });
    expect(sync().written, isEmpty);
  });

  test('codex and antigravity share .agents/skills, written once', () {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [codex, antigravity]\n');

    final result = sync();

    expect(result.written.where((path) => path == '.agents/skills/plan/SKILL.md'), hasLength(1));
    expect(exists('.codex/agents/flutter-planner.toml'), isTrue);
    expect(exists('.agents/agents/flutter-planner.md'), isTrue);
    expect(sync().written, isEmpty);
  });

  test('both agents share AGENTS.md and the guard policy', () {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code, codex]\n');

    sync();

    expect(exists('.claude/skills/plan/SKILL.md'), isTrue);
    expect(exists('.agents/skills/plan/SKILL.md'), isTrue);
    expect(read('.claude/settings.json'), contains('.ai/bin/guard'));
    expect(read('.codex/hooks.json'), contains('.ai/bin/guard'));
    expect(sync().written, isEmpty);
  });

  group('dashcrew version in the lock', () {
    Map<String, Object?> lock() => jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
    void setLockVersion(String? version) {
      final current = lock();
      if (version == null) {
        current.remove('dashcrew_version');
      } else {
        current['dashcrew_version'] = version;
      }
      write('.ai/lock.json', jsonEncode(current));
    }

    test('sync records the version that wrote the lock', () {
      sync();

      expect(lock()['dashcrew_version'], dashcrewVersion);
    });

    test('a lock from a newer dashcrew stops sync until --force', () {
      sync();
      setLockVersion('999.0.0');

      expect(sync, throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('999.0.0'))));

      sync(force: true);
      expect(lock()['dashcrew_version'], dashcrewVersion);
    });

    test('a lock from an older dashcrew, or with no version, is upgraded', () {
      sync();
      setLockVersion('0.0.0');
      sync();
      expect(lock()['dashcrew_version'], dashcrewVersion);

      setLockVersion(null);
      sync();
      expect(lock()['dashcrew_version'], dashcrewVersion);
    });
  });

  group('pull request template', () {
    test('sync writes the template with every required section', () {
      sync();

      final template = read('.github/pull_request_template.md');
      expect(template, startsWith('<!-- dashcrew:generated'));
      expect(template, contains('## Analytics'));
      final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
      expect((lock['files'] as Map).keys, contains('.github/pull_request_template.md'));
    });

    test('a template the team wrote is left alone with a warning', () {
      write('.github/pull_request_template.md', '## Our own\n');

      final result = sync();

      expect(read('.github/pull_request_template.md'), '## Our own\n');
      expect(result.warnings.join('\n'), contains('pull_request_template.md'));
      final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
      expect((lock['files'] as Map).keys, isNot(contains('.github/pull_request_template.md')));
    });
  });

  group('links in the project', () {
    late Directory outside;

    setUp(() {
      outside = Directory.systemTemp.createTempSync('dashcrew_link_target_');
      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\n');
    });
    tearDown(() => outside.deleteSync(recursive: true));

    File outsideFile(String name, String content) => File('${outside.path}/$name')
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(content);

    void link(String relative, String target) {
      Directory('${project.path}/$relative').parent.createSync(recursive: true);
      Link('${project.path}/$relative').createSync(target);
    }

    test('a generated file that is a link is reported, and --force never writes into its target', () {
      final source = outsideFile('planner.md', 'team planner\n');
      link('.claude/agents/flutter-planner.md', source.path);

      expect(() => sync(), throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('link to ${source.path}'))));
      expect(source.readAsStringSync(), 'team planner\n');

      sync(force: true);

      expect(source.readAsStringSync(), 'team planner\n');
      expect(FileSystemEntity.isLinkSync('${project.path}/.claude/agents/flutter-planner.md'), isFalse);
      expect(read('.claude/agents/flutter-planner.md'), contains('name: flutter-planner'));
    });

    test('a generated file inside a linked folder is reported, and --force leaves the folder target alone', () {
      final source = outsideFile('skills/verify/SKILL.md', 'team verify\n');
      link('.claude/skills/verify', source.parent.path);

      expect(() => sync(), throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('.claude/skills/verify'))));

      sync(force: true);

      expect(source.readAsStringSync(), 'team verify\n');
      expect(FileSystemEntity.isLinkSync('${project.path}/.claude/skills/verify'), isFalse);
      expect(read('.claude/skills/verify/SKILL.md'), contains('name: verify'));
    });

    test('a merged file that is a link is merged into a real file, its target untouched', () {
      final source = outsideFile('settings.json', '{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]}}');
      link('.claude/settings.json', source.path);

      sync();

      expect(source.readAsStringSync(), contains('"notify"'));
      expect(source.readAsStringSync(), isNot(contains('.ai/bin/guard')));
      expect(FileSystemEntity.isLinkSync('${project.path}/.claude/settings.json'), isFalse);
      expect(read('.claude/settings.json'), allOf(contains('notify'), contains('.ai/bin/guard')));
    });
  });

  group('github workflow', () {
    const path = '.github/workflows/dashcrew.yml';

    test('ci.github writes a workflow that runs dashcrew check at the pinned ref', () {
      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n');

      sync();

      final workflow = loadYaml(read(path)) as YamlMap;
      expect((workflow['on'] as YamlMap).keys, ['pull_request']);
      final steps = ((workflow['jobs'] as YamlMap)['check'] as YamlMap)['steps'] as YamlList;
      final runs = [for (final step in steps) (step as YamlMap)['run']].whereType<String>().toList();
      expect(runs, contains(contains('--git-ref 0123abc')));
      expect(runs.last, 'dart pub global run dashcrew check');
      expect(read(path), contains(r'GITHUB_TOKEN: ${{ github.token }}'));
      expect(((jsonDecode(read('.ai/lock.json')) as Map)['files'] as Map).keys, contains(path));
    });

    test('no workflow without ci.github, and turning it off removes it', () {
      sync();
      expect(exists(path), isFalse);

      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n');
      sync();
      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\n');
      final result = sync();

      expect(exists(path), isFalse);
      expect(result.removed, contains(path));
    });

    test('a workflow the team wrote under that name is left alone with a warning', () {
      write(path, 'name: ours\n');
      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n');

      final result = sync();

      expect(read(path), 'name: ours\n');
      expect(result.warnings.join('\n'), contains(path));
    });
  });

  group('guard shim', () {
    test('sync writes an executable shim for this dashcrew and locks it', () {
      sync();

      final hash = Sync.guardSourceHash(dashcrewPackageRoot());
      expect(read('.ai/bin/guard'), GuardInstaller.shim(hash));
      expect(File('${project.path}/.ai/bin/guard').statSync().mode & 0x49, 0x49);
      final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
      expect((lock['files'] as Map).keys, contains('.ai/bin/guard'));
      expect(lock['guard_source'], hash);
      expect(exists('.gitignore'), isFalse);
    });

    test('the .ai/bin/ ignore line dashcrew added is removed, other lines stay', () {
      write('.gitignore', 'build/\n# dashcrew: compiled guard, per machine\n.ai/bin/\n.env\n');

      sync();

      expect(read('.gitignore'), 'build/\n.env\n');
    });

    test('a compiled guard left by an older dashcrew is replaced without --force', () {
      File('${project.path}/.ai/bin/guard')
        ..parent.createSync(recursive: true)
        ..writeAsBytesSync([0x7f, 0x45, 0x4c, 0x46, 0x00, 0xff, 0xfe]);

      sync();

      expect(read('.ai/bin/guard'), startsWith('#!/bin/sh'));
    });

    test('compileGuard installs the guard into the guard home', () {
      final home = Directory.systemTemp.createTempSync('dashcrew_sync_home_');
      addTearDown(() => home.deleteSync(recursive: true));

      Sync(catalog: catalog, projectDir: project.path, packageRoot: '.', guardHome: home.path).run();

      final installed = GuardInstaller(packageRoot: '.', home: home.path).binaryPath();
      expect(File(installed).existsSync(), isTrue);
    }, timeout: const Timeout(Duration(minutes: 3)));
  });

  group('project packs', () {
    void writeArchitecturePack() {
      write('.ai/packs/architecture/clean/pack.yaml', 'description: clean\nparams: { domain_dir: lib/domain }\n');
      write('.ai/packs/architecture/clean/rules.md', '---\ntarget: agents_md\norder: 380\n---\n## Clean layers\n\nUse cases live in `{{pack.architecture.domain_dir}}`.\n');
    }

    test('project family renders and the lock records pack sources', () {
      writeArchitecturePack();
      write('.ai/packs/state/bloc/pack.yaml', 'description: team bloc\n');
      write('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\norder: 300\n---\n## State: team bloc\n');
      write('.ai/project.yaml', '''
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  di: get_it
  router: { use: other, name: go_router }
  architecture: clean
''');

      final result = sync();

      expect(result.warnings, contains('skill bloc-patterns skipped: no selected pack defines skills/bloc-patterns.yaml'));
      expect(exists('.claude/skills/bloc-patterns/SKILL.md'), isFalse);
      final agentsMd = read('AGENTS.md');
      expect(agentsMd, contains('Use cases live in `lib/domain`.'));
      expect(agentsMd, contains('## State: team bloc'));
      expect(agentsMd, isNot(contains('## State: flutter_bloc')));
      final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
      expect(lock['packs'], {'architecture/clean': 'project', 'di/get_it': 'builtin', 'state/bloc': 'project'});
    });

    test('project family without its pack is an unknown family', () {
      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nstack: { architecture: clean }\n');

      expect(sync, throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('stack.architecture'))));
    });

    test('broken project pack stops sync with its path', () {
      write('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nx\n');

      expect(sync, throwsA(isA<SyncException>().having((e) => e.message, 'message', contains('.ai/packs/state/bloc/pack.yaml'))));
    });
  });
}
