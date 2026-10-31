import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';

/// PreToolUse hook entry for Claude Code, Codex and Antigravity: hook JSON on stdin, answer per [hookResponse].
/// Policy path: first argument, else `DASHCREW_GUARD_POLICY`, else `<hook cwd or first workspace path>/.ai/guard.json`.
/// Any read or parse failure allows the call; `dashcrew doctor` reports a broken install.
Future<void> main(List<String> arguments) async {
  try {
    final input = jsonDecode(await stdin.transform(utf8.decoder).join());
    if (input is! Map<String, Object?>) exit(0);

    final policyPath = arguments.isNotEmpty
        ? arguments.first
        : Platform.environment['DASHCREW_GUARD_POLICY'] ??
            '${input['cwd'] as String? ?? _firstWorkspace(input) ?? Directory.current.path}/.ai/guard.json';
    final policyFile = File(policyPath);
    if (!policyFile.existsSync()) exit(0);
    final policy = GuardPolicy.fromJson(jsonDecode(policyFile.readAsStringSync()) as Map<String, Object?>);

    final gate = policy.planGate ? readPlanGateState(File(policyPath).absolute.parent.parent.path) : null;
    final response = hookResponse(input, evaluateAll(GuardEvent.allFromHook(input), policy, gate: gate));
    if (response.stdout.isNotEmpty) stdout.writeln(response.stdout);
    if (response.stderr.isNotEmpty) stderr.writeln(response.stderr);
    await stdout.flush();
    exit(response.exitCode);
  } catch (_) {
    exit(0);
  }
}

/// Antigravity sends `workspacePaths` instead of `cwd`.
String? _firstWorkspace(Map<String, Object?> input) {
  final paths = input['workspacePaths'];
  return paths is List && paths.isNotEmpty && paths.first is String ? paths.first as String : null;
}
