import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Runs each adapter's hook command the way an agent does, through `sh -c`, against a stub guard.
void main() {
  late String root;
  late String calls;

  setUp(() {
    root = Directory.systemTemp.createTempSync('dashcrew_hook_command_').resolveSymbolicLinksSync();
    Process.runSync('git', ['init', '-q'], workingDirectory: root);
    calls = p.join(root, 'calls.txt');
    final guard = File(p.join(root, '.ai', 'bin', 'guard'))
      ..parent.createSync(recursive: true)
      ..writeAsStringSync('#!/bin/sh\ncat > /dev/null\necho "called \$*" >> "$calls"\nexit 2\n');
    Process.runSync('chmod', ['+x', guard.path]);
    Directory(p.join(root, 'lib', 'feature')).createSync(recursive: true);
  });
  tearDown(() => Directory(root).deleteSync(recursive: true));

  ProcessResult run(String command, {required String cwd, Map<String, String>? environment}) =>
      Process.runSync('sh', ['-c', command], workingDirectory: cwd, environment: environment);

  String policy() => p.join(root, '.ai', 'guard.json');

  test('claude code reaches the guard from a subfolder through CLAUDE_PROJECT_DIR', () {
    final result = run(ClaudeCodeAdapter.guardCommand, cwd: p.join(root, 'lib', 'feature'), environment: {'CLAUDE_PROJECT_DIR': root});

    expect(result.exitCode, 2, reason: '${result.stderr}');
    expect(File(calls).readAsStringSync(), 'called ${policy()}\n');
  });

  test('codex reaches the guard from a subfolder through git rev-parse', () {
    final result = run(CodexAdapter.guardCommand, cwd: p.join(root, 'lib', 'feature'));

    expect(result.exitCode, 2, reason: '${result.stderr}');
    expect(File(calls).readAsStringSync(), 'called ${policy()}\n');
  });

  test('antigravity reaches the guard from the workspace root', () {
    final command = (((AntigravityAdapter.guardHook['PreToolUse'] as List).single as Map)['hooks'] as List).single as Map;

    final result = run(command['command'] as String, cwd: root);

    expect(result.exitCode, 2, reason: '${result.stderr}');
    expect(File(calls).readAsStringSync(), 'called \n');
  });
}
