import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory project;
  late Directory userSkills;
  final catalog = Catalog.load('.');

  String read(String relative) => File(p.join(project.path, relative)).readAsStringSync();
  void write(String relative, String content) {
    final file = File(p.join(project.path, relative));
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(content);
  }

  void delete(String relative) => File(p.join(project.path, relative)).deleteSync();

  void sync({bool compileGuard = false}) =>
      Sync(catalog: catalog, projectDir: project.path, compileGuard: compileGuard, packageRoot: '.').run();

  Future<Map<String, DoctorCheck>> doctor() async {
    final checks = await Doctor(
      catalog: catalog,
      projectDir: project.path,
      packageRoot: '.',
      userSkillsDir: userSkills.path,
    ).run();
    return {for (final check in checks) check.id: check};
  }

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_doctor_');
    userSkills = Directory.systemTemp.createTempSync('dashcrew_doctor_skills_');
    write('.ai/project.yaml', '''
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  di: get_it
''');
  });
  tearDown(() {
    project.deleteSync(recursive: true);
    userSkills.deleteSync(recursive: true);
  });

  test('fresh sync with a compiled guard passes every check', () async {
    sync(compileGuard: true);

    final checks = await doctor();

    for (final check in checks.values) {
      if (check.id == 'skills') {
        expect(check.status, isNot(CheckStatus.fail), reason: check.detail);
      } else {
        expect(check.status, CheckStatus.ok, reason: '${check.id}: ${check.detail}');
      }
    }
    expect(checks.keys, containsAll(['profile', 'lock', 'drift', 'stale', 'guard-policy', 'guard-binary', 'hook']));
  }, timeout: const Timeout(Duration(minutes: 2)));

  test('guard not compiled fails guard-binary', () async {
    sync();

    final checks = await doctor();

    expect(checks['guard-binary']!.status, CheckStatus.fail);
    expect(checks['guard-binary']!.detail, contains('missing'));
  });

  test('generated file edited by hand fails drift', () async {
    sync();
    write('.claude/skills/plan/SKILL.md', 'edited by hand\n');

    final checks = await doctor();

    expect(checks['drift']!.status, CheckStatus.fail);
    expect(checks['drift']!.detail, contains('.claude/skills/plan/SKILL.md'));
  });

  test('generated file deleted fails drift', () async {
    sync();
    delete('.claude/agents/flutter-planner.md');

    final checks = await doctor();

    expect(checks['drift']!.status, CheckStatus.fail);
    expect(checks['drift']!.detail, contains('.claude/agents/flutter-planner.md'));
  });

  test('editing the AGENTS.md project block is not drift', () async {
    sync();
    write('AGENTS.md', read('AGENTS.md').replaceFirst(
      '<!-- dashcrew:project:begin -->',
      '<!-- dashcrew:project:begin -->\nTeam rule.',
    ));

    final checks = await doctor();

    expect(checks['drift']!.status, CheckStatus.ok);
    expect(checks['stale']!.status, CheckStatus.ok);
  });

  test('profile changed after sync warns stale without drift', () async {
    sync();
    write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\n');

    final checks = await doctor();

    expect(checks['stale']!.status, CheckStatus.warn);
    expect(checks['stale']!.detail, contains('dashcrew sync'));
    expect(checks['drift']!.status, CheckStatus.ok);
  });

  test('missing guard.json fails guard-policy', () async {
    sync();
    delete('.ai/guard.json');

    final checks = await doctor();

    expect(checks['guard-policy']!.status, CheckStatus.fail);
  });

  test('guard hook removed from settings fails hook', () async {
    sync();
    write('.claude/settings.json', jsonEncode({'hooks': <String, Object?>{}}));

    final checks = await doctor();

    expect(checks['hook']!.status, CheckStatus.fail);
  });

  test('CLAUDE.md without the AGENTS.md import fails claude-md', () async {
    write('CLAUDE.md', 'Personal notes.\n');
    sync();

    final checks = await doctor();

    expect(checks['claude-md']!.status, CheckStatus.fail);
  });

  test('missing dashcrew skill fails skills', () async {
    sync();
    delete('.claude/skills/prompt-polish/SKILL.md');

    final checks = await doctor();

    expect(checks['skills']!.status, CheckStatus.fail);
    expect(checks['skills']!.detail, contains('prompt-polish'));
  });

  test('unknown external skill warns, one in the user skills dir does not', () async {
    sync();
    write('AGENTS.md', read('AGENTS.md').replaceFirst(
      '<!-- dashcrew:project:begin -->',
      '<!-- dashcrew:project:begin -->\nUse the `made-up-skill` skill and skill `user-installed`.',
    ));
    File(p.join(userSkills.path, 'user-installed', 'SKILL.md'))
      ..parent.createSync(recursive: true)
      ..writeAsStringSync('---\nname: user-installed\n---\n');

    final checks = await doctor();

    expect(checks['skills']!.status, CheckStatus.warn);
    expect(checks['skills']!.detail, contains('made-up-skill'));
    expect(checks['skills']!.detail, isNot(contains('user-installed')));
  });

  test('missing .ai/bin/ in .gitignore warns', () async {
    sync();
    write('.gitignore', 'build/\n');

    final checks = await doctor();

    expect(checks['gitignore']!.status, CheckStatus.warn);
  });

  test('never synced: lock fails and dependent checks are skipped', () async {
    final checks = await doctor();

    expect(checks['profile']!.status, CheckStatus.ok);
    expect(checks['lock']!.status, CheckStatus.fail);
    expect(checks['drift']!.status, CheckStatus.skip);
    expect(checks['stale']!.status, CheckStatus.skip);
  });

  test('missing profile fails profile and skips stale', () async {
    sync();
    delete('.ai/project.yaml');

    final checks = await doctor();

    expect(checks['profile']!.status, CheckStatus.fail);
    expect(checks['stale']!.status, CheckStatus.skip);
  });

  group('version', () {
    void setLockVersion(String? version) {
      final lock = jsonDecode(read('.ai/lock.json')) as Map<String, Object?>;
      if (version == null) {
        lock.remove('dashcrew_version');
      } else {
        lock['dashcrew_version'] = version;
      }
      write('.ai/lock.json', jsonEncode(lock));
    }

    test('ok right after sync', () async {
      sync();

      expect((await doctor())['version']!.status, CheckStatus.ok);
    });

    test('warns for a newer, older or missing lock version', () async {
      sync();
      for (final (version, hint) in [('999.0.0', 'update dashcrew'), ('0.0.0', 'dashcrew sync'), (null, 'dashcrew sync')]) {
        setLockVersion(version);

        final check = (await doctor())['version']!;

        expect(check.status, CheckStatus.warn, reason: '$version');
        expect(check.detail, contains(hint), reason: '$version');
      }
    });
  });

  group('project-packs', () {
    test('no project packs is ok', () async {
      sync();

      expect((await doctor())['project-packs']!.status, CheckStatus.ok);
    });

    test('a pack that does not parse fails', () async {
      sync();
      write('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nx\n');

      final checks = await doctor();

      expect(checks['project-packs']!.status, CheckStatus.fail);
      expect(checks['project-packs']!.detail, contains('.ai/packs/state/bloc/pack.yaml'));
    });

    test('a pack the profile does not select warns', () async {
      write('.ai/packs/architecture/clean/pack.yaml', 'description: clean\n');
      sync();

      final checks = await doctor();

      expect(checks['project-packs']!.status, CheckStatus.warn);
      expect(checks['project-packs']!.detail, contains('architecture/clean'));
    });

    test('selected project pack is ok and editing it turns stale', () async {
      write('.ai/packs/state/bloc/pack.yaml', 'description: team bloc\n');
      write('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nTeam Bloc.\n');
      sync();

      expect((await doctor())['project-packs']!.status, CheckStatus.ok);
      expect((await doctor())['stale']!.status, CheckStatus.ok);

      write('.ai/packs/state/bloc/rules.md', '---\ntarget: agents_md\n---\nTeam Bloc, edited.\n');
      final checks = await doctor();

      expect(checks['stale']!.status, CheckStatus.warn);
      expect(checks['drift']!.status, CheckStatus.ok);
    });
  });

  group('codex', () {
    late Directory codexHome;

    Future<Map<String, DoctorCheck>> codexDoctor() async {
      final checks = await Doctor(
        catalog: catalog,
        projectDir: project.path,
        packageRoot: '.',
        userSkillsDir: userSkills.path,
        codexHome: codexHome.path,
      ).run();
      return {for (final check in checks) check.id: check};
    }

    void mergeCodex() => CodexAdapter(catalog, projectDir: project.path).merge(const {}, <String>[]);

    setUp(() {
      codexHome = Directory.systemTemp.createTempSync('dashcrew_doctor_codex_');
      write('.ai/project.yaml', 'toolkit: 1\nagents: [codex]\n');
    });
    tearDown(() => codexHome.deleteSync(recursive: true));

    test('only the checks of the selected agents run', () async {
      final codexOnly = await codexDoctor();
      expect(codexOnly.keys, containsAll(['codex-hook', 'codex-mcp', 'codex-skills-budget', 'codex-trust']));
      expect(codexOnly.keys, isNot(contains('claude-md')));

      write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\n');
      final claudeOnly = await codexDoctor();
      expect(claudeOnly.keys, contains('claude-md'));
      expect(claudeOnly.keys, isNot(contains('codex-hook')));
    });

    test('hook and mcp fail or warn until merged, then pass', () async {
      final before = await codexDoctor();
      expect(before['codex-hook']!.status, CheckStatus.fail);
      expect(before['codex-mcp']!.status, CheckStatus.warn);

      mergeCodex();
      final after = await codexDoctor();
      expect(after['codex-hook']!.status, CheckStatus.ok);
      expect(after['codex-mcp']!.status, CheckStatus.ok);
    });

    test('an mcp server the user defines is fine', () async {
      write('.codex/config.toml', '[mcp_servers.dart-mcp-server]\ncommand = "fvm"\n');

      final check = (await codexDoctor())['codex-mcp']!;

      expect(check.status, CheckStatus.ok);
      expect(check.detail, contains('user'));
    });

    test('a skill list over the budget warns', () async {
      write('.agents/skills/small/SKILL.md', '---\nname: small\ndescription: "short"\n---\n');
      expect((await codexDoctor())['codex-skills-budget']!.status, CheckStatus.ok);

      write('.agents/skills/huge/SKILL.md', '---\nname: huge\ndescription: "${'x' * 8000}"\n---\n');
      expect((await codexDoctor())['codex-skills-budget']!.status, CheckStatus.warn);
    });

    test('trust is read from the codex config of the user', () async {
      expect((await codexDoctor())['codex-trust']!.status, CheckStatus.warn);

      File('${codexHome.path}/config.toml').writeAsStringSync('model = "m"\n\n[projects."${project.path}"]\ntrust_level = "trusted"\n');
      expect((await codexDoctor())['codex-trust']!.status, CheckStatus.ok);
    });
  });
  group('antigravity', () {
    setUp(() => write('.ai/project.yaml', 'toolkit: 1\nagents: [antigravity]\n'));

    test('a fresh sync passes the antigravity checks', () async {
      sync();

      final checks = await doctor();

      for (final id in ['antigravity-hook', 'antigravity-mcp', 'antigravity-roles']) {
        expect(checks[id]!.status, CheckStatus.ok, reason: '$id: ${checks[id]!.detail}');
      }
      expect(checks.keys, isNot(contains('claude-md')));
      expect(checks['agents-md-size']!.detail, contains('24000'));
    });

    test('a removed guard entry fails and an unknown role tool fails', () async {
      sync();
      write('.agents/hooks.json', '{}');
      write('.agents/agents/flutter-planner.md', read('.agents/agents/flutter-planner.md').replaceFirst('view_file', 'view_files'));

      final checks = await doctor();

      expect(checks['antigravity-hook']!.status, CheckStatus.fail);
      expect(checks['antigravity-roles']!.status, CheckStatus.fail);
      expect(checks['antigravity-roles']!.detail, contains('view_files'));
    });

    test('AGENTS.md over 24000 bytes warns when antigravity is selected', () async {
      sync();
      write('AGENTS.md', read('AGENTS.md').replaceFirst(
        '<!-- dashcrew:project:begin -->',
        '<!-- dashcrew:project:begin -->\n${'x' * 9000}',
      ));

      expect((await doctor())['agents-md-size']!.status, CheckStatus.warn);
    });
  });
}
