import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  final catalog = Catalog.load('.');
  late Directory project;
  final asked = <int>[];
  var comments = <Map<String, Object?>>[];

  void write(String relative, String content) => File(p.join(project.path, relative))
    ..parent.createSync(recursive: true)
    ..writeAsStringSync(content);

  void sync(String policy) {
    write('.ai/project.yaml', 'toolkit: 1\nagents: [claude-code]\nstack: { qa: semantics-identifier, analytics: posthog }\n$policy');
    Sync(catalog: catalog, projectDir: project.path, compileGuard: false).run(force: true);
  }

  Future<Map<String, DoctorCheck>> check({String? body, Map<String, String> environment = const {}}) async {
    final checks = await PrCheck(
      catalog: catalog,
      projectDir: project.path,
      comments: (issue) async {
        asked.add(issue);
        return comments;
      },
      environment: environment,
    ).run(prBody: body);
    return {for (final c in checks) c.id: c};
  }

  const fullBody = '## Summary\nx\n## Changes\nx\n## Affected files\nx\n## Testing\nx\n## QA Identifiers\nNone\n## Analytics\nNone\n## Notes / not done\nNone\n';

  setUp(() {
    project = Directory.systemTemp.createTempSync('dashcrew_check_');
    asked.clear();
    comments = [];
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

  test('a named plan issue must be approved', () async {
    final body = '$fullBody\nPlan: #42\n';
    comments = [
      {'body': '${Gates.planMarker}\nv1', 'author_association': 'COLLABORATOR', 'created_at': '2026-10-31T09:00:00Z'},
    ];
    final pending = (await check(body: body))['plan-approval']!;
    expect(pending.status, CheckStatus.fail);
    expect(pending.detail, contains('#42'));
    expect(asked, [42]);

    comments.add({'body': '/approve plan', 'author_association': 'MEMBER', 'created_at': '2026-10-31T09:05:00Z'});
    expect((await check(body: body))['plan-approval']!.status, CheckStatus.ok);
  });

  test('without plan_gate a PR may skip the plan line; with it the line is required', () async {
    expect((await check(body: fullBody))['plan-approval']!.status, CheckStatus.skip);

    sync('policy: { plan_gate: true }\n');
    final required = (await check(body: fullBody))['plan-approval']!;
    expect(required.status, CheckStatus.fail);
    expect(required.detail, contains('Plan: #'));
  });

  test('in a GitHub Action the PR body comes from the event file', () async {
    final event = File(p.join(project.path, 'event.json'))
      ..writeAsStringSync(jsonEncode({
        'pull_request': {'body': fullBody},
      }));

    final checks = await check(environment: {'GITHUB_EVENT_PATH': event.path});

    expect(checks['pr-sections']!.status, CheckStatus.ok);
  });

  test('in CI the issue comments come from the REST API, page by page, with the token', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final seen = <String>[];
    server.listen((request) async {
      seen.add('${request.uri.path}?${request.uri.query} ${request.headers.value('authorization')}');
      final page = int.parse(request.uri.queryParameters['page']!);
      final batch = page == 1
          ? [
              {'body': '${Gates.planMarker}\nv1', 'author_association': 'OWNER', 'created_at': 'a', 'user': {'login': 'x'}},
              for (var i = 0; i < 99; i++) {'body': 'chat $i', 'author_association': 'NONE', 'created_at': 'b', 'user': {'login': 'y'}},
            ]
          : [
              {'body': '/approve plan', 'author_association': 'MEMBER', 'created_at': 'c', 'user': {'login': 'lead'}},
            ];
      request.response
        ..headers.contentType = ContentType.json
        ..write(jsonEncode(batch));
      await request.response.close();
    });

    final checks = await PrCheck(
      catalog: catalog,
      projectDir: project.path,
      environment: {
        'GITHUB_TOKEN': 'secret-token',
        'GITHUB_REPOSITORY': 'team/app',
        'GITHUB_API_URL': 'http://127.0.0.1:${server.port}',
      },
    ).run(prBody: '$fullBody\nPlan: #42\n');

    expect({for (final c in checks) c.id: c.status}['plan-approval'], CheckStatus.ok);
    expect(seen, [
      '/repos/team/app/issues/42/comments?per_page=100&page=1 Bearer secret-token',
      '/repos/team/app/issues/42/comments?per_page=100&page=2 Bearer secret-token',
    ]);
  });
}
