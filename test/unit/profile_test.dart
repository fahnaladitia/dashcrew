import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  const knownOptions = {
    'state': {'bloc', 'riverpod'},
    'di': {'get_it', 'none'},
    'analytics': {'posthog', 'none'},
  };

  Profile parse(String yaml) => Profile.parse(yaml, knownOptions: knownOptions);

  test('minimal profile gets defaults', () {
    final profile = parse('toolkit: 1\nagents: [claude-code]\n');

    expect(profile.agents, ['claude-code']);
    expect(profile.language['dartdoc'], 'en');
    expect(profile.policy['agent_commit'], false);
    expect(profile.policy['dartdoc_max_lines'], 3);
    expect(profile.policy['plan_gate'], false);
    expect(profile.paths['plans'], '.ai/plans');
    expect(profile.stack, isEmpty);
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
}
