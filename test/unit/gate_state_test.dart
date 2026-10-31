import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  late Directory root;
  late String repo;

  String git(String dir, List<String> args) {
    final result = Process.runSync('git', ['-c', 'user.name=t', '-c', 'user.email=t@example.com', ...args], workingDirectory: dir);
    if (result.exitCode != 0) fail('git ${args.join(' ')}: ${result.stderr}');
    return (result.stdout as String).trim();
  }

  void writeStates(Map<String, Object?> states) => File(p.join(repo, '.git', 'dashcrew', 'gates.json'))
    ..parent.createSync(recursive: true)
    ..writeAsStringSync(jsonEncode(states));

  setUp(() {
    root = Directory(Directory.systemTemp.createTempSync('dashcrew_gate_state_').resolveSymbolicLinksSync());
    repo = p.join(root.path, 'app');
    Directory(repo).createSync();
    git(repo, ['init', '-q', '-b', 'main']);
    git(repo, ['commit', '-q', '--allow-empty', '-m', 'init']);
  });
  tearDown(() => root.deleteSync(recursive: true));

  test('reads the current branch and its recorded plan state without running git', () {
    git(repo, ['checkout', '-q', '-b', 'feat/orders']);
    writeStates({
      'feat/orders': {'issue': 42, 'plan': 'pending', 'at': null},
    });

    final state = readPlanGateState(repo);

    expect(state, (branch: 'feat/orders', plan: 'pending', issue: 42));
  });

  test('a worktree reads the shared state of its own branch', () {
    git(repo, ['worktree', 'add', '-q', '-b', 'feat/profile', p.join(root.path, 'wt'), 'main']);
    writeStates({
      'feat/profile': {'issue': 7, 'plan': 'approved', 'at': '2026-10-31T09:00:00Z'},
    });

    expect(readPlanGateState(p.join(root.path, 'wt')), (branch: 'feat/profile', plan: 'approved', issue: 7));
  });

  test('null without a recorded branch, on a detached HEAD, or outside git', () {
    expect(readPlanGateState(repo), isNull);
    git(repo, ['checkout', '-q', '--detach']);
    writeStates({'main': {'issue': 1, 'plan': 'pending'}});
    expect(readPlanGateState(repo), isNull);
    expect(readPlanGateState(root.path), isNull);
  });
}
