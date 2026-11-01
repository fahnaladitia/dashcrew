import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');
  late Directory project;

  void write(String relative, String content) => File(p.join(project.path, relative))
    ..parent.createSync(recursive: true)
    ..writeAsStringSync(content);

  void sync(String policy) {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nstack: { qa: semantics-identifier, analytics: posthog }\n$policy');
    Sync(catalog: catalog, projectDir: project.path, compileGuard: false).run(force: true);
  }

  Future<Map<String, DoctorCheck>> check({String? body, Map<String, String> environment = const {}}) async {
    final checks = await PrCheck(catalog: catalog, projectDir: project.path, environment: environment).run(prBody: body);
    return {for (final c in checks) c.id: c};
  }

  const fullBody = '## Summary\nx\n## Changes\nx\n## Affected files\nx\n## Testing\nx\n## QA Identifiers\nNone\n## Analytics\nNone\n## Notes / not done\nNone\n';

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_check_');
    sync('');
  });
  tearDown(() => project.deleteSync(recursive: true));

  test('toolkit checks run without the machine and agent checks', () async {
    final checks = await check(body: fullBody);

    expect(checks['drift']!.status, CheckStatus.ok);
    expect(checks['stale']!.status, CheckStatus.ok);
    expect(checks.keys, isNot(contains('guard-binary')));
    expect(checks.keys, isNot(contains('hook')));
  });

  test('every template heading must be in the PR body', () async {
    expect((await check(body: fullBody))['pr-sections']!.status, CheckStatus.ok);

    final missing = (await check(body: fullBody.replaceFirst('## Analytics\nNone\n', '')))['pr-sections']!;
    expect(missing.status, CheckStatus.fail);
    expect(missing.detail, contains('## Analytics'));
  });

  test('no PR body skips the section check; a missing template fails it', () async {
    expect((await check())['pr-sections']!.status, CheckStatus.skip);

    File(p.join(project.path, Sync.prTemplatePath)).deleteSync();
    expect((await check(body: fullBody))['pr-sections']!.status, CheckStatus.fail);
  });

  test('a PR body with no Plan: line passes, even when the profile still sets plan_gate', () async {
    sync('policy: { plan_gate: true }\n');

    final checks = await check(body: fullBody);

    expect(checks.keys, isNot(contains('plan-approval')));
    expect(checks['pr-sections']!.status, CheckStatus.ok);
    expect([for (final c in checks.values) if (c.status == CheckStatus.fail) c.id], isEmpty);
  });

  test('in a GitHub Action the PR body comes from the event file', () async {
    final event = File(p.join(project.path, 'event.json'))
      ..writeAsStringSync(jsonEncode({
        'pull_request': {'body': fullBody},
      }));

    final checks = await check(environment: {'GITHUB_EVENT_PATH': event.path});

    expect(checks['pr-sections']!.status, CheckStatus.ok);
  });
}
