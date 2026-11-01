import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');

  GuardPolicy compile(String yaml) {
    final profile = Profile.parse(yaml, knownOptions: catalog.knownOptions());
    return compileGuardPolicy(profile, catalog.resolvedFor(profile.platform));
  }

  test('defaults block commit and formatters, plans under .ai/plans', () {
    final policy = compile('toolkit: 1\nagents: [claude-code]\n');

    expect(policy.blockedGit, containsAll(['commit', 'push', 'stash']));
    expect(policy.blockFormatters, isTrue);
    expect(policy.plansDir, '.ai/plans');
    expect(policy.forbiddenImports, isEmpty);
  });

  test('role capabilities come from role.yaml: the two subagents are read-only', () {
    final policy = compile('toolkit: 1\nagents: [claude-code]\n');

    expect(policy.roleCapability, {'explorer': 'read', 'reviewer': 'read'});
  });

  test('agent_commit and formatter policies relax only their own rules', () {
    final policy = compile('toolkit: 1\nagents: [claude-code]\npolicy: { agent_commit: true, formatter: true }\n');

    expect(policy.blockedGit, isNot(contains('commit')));
    expect(policy.blockedGit, contains('push'));
    expect(policy.blockFormatters, isFalse);
  });

  test('analytics pack vendor import and local packages flow into the policy', () {
    final policy = compile('''
toolkit: 1
agents: [claude-code]
stack: { analytics: posthog }
project: { local_packages: [core] }
paths: { plans: docs/plans }
''');

    expect(policy.forbiddenImports, ['package:posthog_flutter']);
    expect(policy.localPackages, ['core']);
    expect(policy.plansDir, 'docs/plans');
  });
}
