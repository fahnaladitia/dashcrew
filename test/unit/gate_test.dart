import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// `gh` stand-in: records every call and answers from canned results.
class FakeGh {
  final calls = <({List<String> args, String? body})>[];
  final comments = <Map<String, Object?>>[];
  var nextIssueUrl = 'https://github.com/team/app/issues/42';
  var failWith = '';

  GhResult call(List<String> args) {
    final file = args.indexOf('--body-file');
    calls.add((args: args, body: file < 0 ? null : File(args[file + 1]).readAsStringSync()));
    if (failWith.isNotEmpty) return (exitCode: 1, stdout: '', stderr: failWith);
    if (args.take(2).join(' ') == 'issue create') return (exitCode: 0, stdout: '$nextIssueUrl\n', stderr: '');
    if (args.take(2).join(' ') == 'issue comment') return (exitCode: 0, stdout: '$nextIssueUrl#issuecomment-1\n', stderr: '');
    if (args.first == 'api') return (exitCode: 0, stdout: comments.map(jsonEncode).join('\n'), stderr: '');
    return (exitCode: 1, stdout: '', stderr: 'unexpected gh ${args.join(' ')}');
  }
}

void main() {
  late Directory repo;
  late FakeGh gh;

  String git(List<String> args) => (Process.runSync('git', args, workingDirectory: repo.path).stdout as String).trim();
  Gates gates() => Gates(projectDir: repo.path, gh: gh.call);
  Map<String, Object?> comment(String body, {String association = 'COLLABORATOR', String at = '2026-10-31T10:00:00Z'}) =>
      {'body': body, 'author_association': association, 'created_at': at, 'login': 'someone'};

  setUp(() {
    repo = Directory(Directory.systemTemp.createTempSync('dashcrew_gate_').resolveSymbolicLinksSync());
    Process.runSync('git', ['init', '-q', '-b', 'feat/orders'], workingDirectory: repo.path);
    File(p.join(repo.path, 'plan.md')).writeAsStringSync('# Plan\n\nStep one.\n');
    gh = FakeGh();
  });
  tearDown(() => repo.deleteSync(recursive: true));

  group('submit plan', () {
    test('creates an issue when none is linked, posts the plan and links the issue', () {
      final result = gates().submitPlan(p.join(repo.path, 'plan.md'));

      expect(result.issue, 42);
      expect(gh.calls.first.args.take(2), ['issue', 'create']);
      expect(gh.calls.first.args, containsAllInOrder(['--title', 'Plan: feat/orders']));
      final post = gh.calls.last;
      expect(post.args.take(4), ['issue', 'comment', '42', '--body-file']);
      expect(post.body, '${Gates.planMarker}\n# Plan\n\nStep one.\n');
      expect(File(post.args[4]).existsSync(), isFalse);
      expect(git(['config', '--get', 'branch.feat/orders.dashcrew-issue']), '42');
      expect(gates().stateOf('feat/orders')?.plan, 'pending');
    });

    test('reuses the linked issue', () {
      git(['config', 'branch.feat/orders.dashcrew-issue', '7']);

      final result = gates().submitPlan(p.join(repo.path, 'plan.md'));

      expect(result.issue, 7);
      expect(gh.calls.map((c) => c.args.take(2).join(' ')), ['issue comment']);
    });

    test('an explicit --issue wins and becomes the link', () {
      gates().submitPlan(p.join(repo.path, 'plan.md'), issue: 9);

      expect(git(['config', '--get', 'branch.feat/orders.dashcrew-issue']), '9');
      expect(gh.calls.single.args.take(3), ['issue', 'comment', '9']);
    });

    test('gh failures and a missing plan file stop with a clear message', () {
      gh.failWith = 'HTTP 401: Bad credentials';
      expect(() => gates().submitPlan(p.join(repo.path, 'plan.md')),
          throwsA(isA<GateException>().having((e) => e.message, 'message', contains('Bad credentials'))));
      expect(() => gates().submitPlan(p.join(repo.path, 'missing.md')),
          throwsA(isA<GateException>().having((e) => e.message, 'message', contains('missing.md'))));
    });
  });

  group('status', () {
    setUp(() => git(['config', 'branch.feat/orders.dashcrew-issue', '42']));

    test('approved by a collaborator after the latest plan', () {
      gh.comments.addAll([
        comment('${Gates.planMarker}\n# Plan', at: '2026-10-31T09:00:00Z'),
        comment('/approve plan\nlooks good', at: '2026-10-31T09:05:00Z'),
      ]);

      final status = gates().status();

      expect(status.plan, 'approved');
      expect(status.issue, 42);
      expect(gates().stateOf('feat/orders')?.plan, 'approved');
      expect(gh.calls.single.args.first, 'api');
    });

    test('an approval before a newer plan does not count', () {
      gh.comments.addAll([
        comment('${Gates.planMarker}\nv1', at: '2026-10-31T09:00:00Z'),
        comment('/approve plan', at: '2026-10-31T09:05:00Z'),
        comment('${Gates.planMarker}\nv2', at: '2026-10-31T09:10:00Z'),
      ]);

      expect(gates().status().plan, 'pending');
    });

    test('only collaborators approve, and only with the exact first line', () {
      gh.comments.addAll([
        comment('${Gates.planMarker}\nv1'),
        comment('/approve plan', association: 'CONTRIBUTOR'),
        comment('/approve plans'),
        comment('ok /approve plan'),
      ]);

      expect(gates().status().plan, 'pending');
    });

    test('no linked issue means no gate and no gh call', () {
      git(['config', '--unset', 'branch.feat/orders.dashcrew-issue']);

      expect(gates().status().plan, 'none');
      expect(gh.calls, isEmpty);
    });

    test('the state file lives in the git common dir, keyed by branch', () {
      gh.comments.addAll([comment('${Gates.planMarker}\nv1'), comment('/approve plan')]);

      gates().status();

      final file = File(p.join(repo.path, '.git', 'dashcrew', 'gates.json'));
      final state = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
      expect((state['feat/orders'] as Map)['plan'], 'approved');
      expect((state['feat/orders'] as Map)['issue'], 42);
    });
  });
}
