import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  const knownOptions = {
    'flutter': {
      'state': {'bloc', 'riverpod'},
      'di': {'get_it', 'none'},
      'analytics': {'posthog', 'none'},
    },
    'go': {'tracker': {'none'}},
    'other': {'tracker': {'none'}},
  };

  Profile parse(String yaml) => Profile.parse(yaml, knownOptions: knownOptions);

  test('minimal profile gets defaults', () {
    final profile = parse('toolkit: 1\nagents: [claude-code]\n');

    expect(profile.agents, ['claude-code']);
    expect(profile.language['dartdoc'], 'en');
    expect(profile.policy['agent_commit'], false);
    expect(profile.policy['dartdoc_max_lines'], 3);
    expect(profile.policy.containsKey('plan_gate'), isFalse);
    expect(profile.warnings, isEmpty);
    expect(profile.paths['plans'], '.ai/plans');
    expect(profile.stack, isEmpty);
  });

  test('policy.plan_gate is still accepted, ignored, and warned about', () {
    final profile = parse('toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n');

    expect(profile.policy.containsKey('plan_gate'), isFalse);
    expect(profile.toContext()['policy'], isNot(contains('plan_gate')));
    expect(profile.warnings, ['plan_gate was removed in P4; delete it from .ai/project.yaml']);
  });

  test('stack accepts shorthand and map form', () {
    final profile = parse('''
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  analytics: { use: posthog, facade: Analytics.instance }
''');

    expect(profile.stack['state']!.option, 'bloc');
    expect(profile.stack['state']!.params, isEmpty);
    expect(profile.stack['analytics']!.option, 'posthog');
    expect(profile.stack['analytics']!.params, {'facade': 'Analytics.instance'});
  });

  test('overrides replace defaults key by key', () {
    final profile = parse('''
toolkit: 1
agents: [claude-code]
language: { dartdoc: id }
paths: { plans: docs/plans }
''');

    expect(profile.language['dartdoc'], 'id');
    expect(profile.language['pr'], 'en');
    expect(profile.paths['plans'], 'docs/plans');
    expect(profile.paths['audits'], '.ai/audits');
  });

  test('unknown top-level key fails with its path', () {
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nstak: {}\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'stak')),
    );
  });

  test('unknown stack option fails with its path', () {
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nstack: { state: mobx }\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'stack.state')),
    );
  });

  test('other is valid for every family and needs a name', () {
    final profile = parse('''
toolkit: 1
agents: [claude-code]
stack:
  state: { use: other, name: mobx }
  di: { use: other, name: injectable }
''');

    expect(profile.stack['state']!.option, 'other');
    expect(profile.stack['state']!.params, {'name': 'mobx'});
    final context = profile.toContext();
    expect(context['stack_other'], [
      {'family': 'di', 'name': 'injectable'},
      {'family': 'state', 'name': 'mobx'},
    ]);
    expect(context['has_stack_other'], true);
  });

  for (final raw in ['other', '{ use: other }', "{ use: other, name: '' }", '{ use: other, name: [a] }']) {
    test('other without a name fails with its path ($raw)', () {
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nstack: { state: $raw }\n'),
        throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'stack.state.name')),
      );
    });
  }

  test('unknown option error lists other', () {
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nstack: { state: mobx }\n'),
      throwsA(isA<ProfileException>().having((e) => e.message, 'message', contains('other'))),
    );
  });

  test('no other choice gives an empty stack_other', () {
    final context = parse('toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n').toContext();

    expect(context['stack_other'], isEmpty);
    expect(context['has_stack_other'], false);
  });

  test('unknown stack family fails with its path', () {
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nstack: { orm: drift }\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'stack.orm')),
    );
  });

  test('missing toolkit version fails', () {
    expect(
      () => parse('agents: [claude-code]\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'toolkit')),
    );
  });

  test('unknown agent fails with its path', () {
    expect(
      () => parse('toolkit: 1\nagents: [cursor]\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'agents')),
    );
  });

  test('flavors become a build list sorted by name', () {
    final context = parse('''
toolkit: 1
agents: [claude-code]
project: { flavors: { staging: lib/main_staging.dart, dev: lib/main_dev.dart } }
''').toContext();

    expect((context['build'] as Map)['flavors'], [
      {'name': 'dev', 'entrypoint': 'lib/main_dev.dart'},
      {'name': 'staging', 'entrypoint': 'lib/main_staging.dart'},
    ]);
    expect((context['build'] as Map)['has_flavors'], true);
  });

  test('no flavors gives an empty build list', () {
    final build = parse('toolkit: 1\nagents: [claude-code]\n').toContext()['build'] as Map;

    expect(build['flavors'], isEmpty);
    expect(build['has_flavors'], false);
  });

  for (final flavors in ['[dev]', '{ dev: [a] }', "{ dev: '' }"]) {
    test('flavors must map a name to an entrypoint ($flavors)', () {
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nproject: { flavors: $flavors }\n'),
        throwsA(isA<ProfileException>().having((e) => e.path, 'path', startsWith('project.flavors'))),
      );
    });
  }

  test('ci defaults to off; github needs a pinned dashcrew ref', () {
    final profile = parse('toolkit: 1\nagents: [claude-code]\n');
    expect(profile.ci, {'github': false, 'dashcrew_ref': null});

    expect(parse('toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n').ci['dashcrew_ref'], '0123abc');
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nci: { github: true }\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'ci.dashcrew_ref')),
    );
    expect(
      () => parse('toolkit: 1\nagents: [claude-code]\nci: { gitlab: true }\n'),
      throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'ci.gitlab')),
    );
  });

  test('toContext exposes nested maps for templates', () {
    final profile = parse('''
toolkit: 1
agents: [claude-code]
stack: { analytics: { use: posthog, facade: Analytics.instance } }
''');

    final context = profile.toContext();

    expect((context['policy'] as Map)['formatter'], false);
    expect(((context['stack'] as Map)['analytics'] as Map)['use'], 'posthog');
  });

  group('platform', () {
    const commands = 'commands: { analyze: make lint, test: make test, test_scoped: make test-one, deps: make deps }';

    test('missing reads as flutter, with no platform_config', () {
      final profile = parse('toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n');

      expect(profile.platform, 'flutter');
      expect(profile.platformConfig, isNull);
    });

    test('a project platform validates the stack against its own families', () {
      final profile = parse('toolkit: 1\nagents: [claude-code]\nplatform: go\nstack: { tracker: none }\n');

      expect(profile.platform, 'go');
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nplatform: go\nstack: { state: bloc }\n'),
        throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'stack.state')),
      );
    });

    test('unknown platform fails and lists the known ones', () {
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nplatform: rust\n'),
        throwsA(isA<ProfileException>()
            .having((e) => e.path, 'path', 'platform')
            .having((e) => e.message, 'message', allOf(contains('flutter'), contains('go'), contains('other')))),
      );
    });

    test('other reads detect and commands from platform_config; format is optional', () {
      final profile = parse('''
toolkit: 1
agents: [claude-code]
platform: other
platform_config:
  detect: [Makefile]
  $commands
''');

      expect(profile.platform, 'other');
      expect(profile.platformConfig, {
        'detect': ['Makefile'],
        'commands': {'analyze': 'make lint', 'test': 'make test', 'test_scoped': 'make test-one', 'deps': 'make deps', 'format': null},
      });
    });

    test('other without platform_config fails', () {
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nplatform: other\n'),
        throwsA(isA<ProfileException>().having((e) => e.path, 'path', 'platform_config')),
      );
    });

    test('platform_config without platform other fails', () {
      expect(
        () => parse('toolkit: 1\nagents: [claude-code]\nplatform_config: { detect: [Makefile], $commands }\n'),
        throwsA(isA<ProfileException>()
            .having((e) => e.path, 'path', 'platform_config')
            .having((e) => e.message, 'message', contains('platform: other'))),
      );
    });

    final broken = {
      'detect: [], $commands': 'platform_config.detect',
      'detect: Makefile, $commands': 'platform_config.detect',
      'detect: [Makefile]': 'platform_config.commands',
      'detect: [Makefile], commands: { analyze: make lint, test: make test, deps: make deps }': 'platform_config.commands.test_scoped',
      "detect: [Makefile], commands: { analyze: '', test: a, test_scoped: b, deps: c }": 'platform_config.commands.analyze',
      'detect: [Makefile], commands: { analyze: a, test: b, test_scoped: c, deps: d, lint: e }': 'platform_config.commands.lint',
      'detect: [Makefile], $commands, link: x': 'platform_config.link',
    };
    broken.forEach((config, path) {
      test('invalid platform_config fails with its path ($path)', () {
        expect(
          () => parse('toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { $config }\n'),
          throwsA(isA<ProfileException>().having((e) => e.path, 'path', path)),
        );
      });
    });
  });
}
