import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

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

    expect(read('.gitignore'), contains('.ai/bin/'));
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
