import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';

/// PreToolUse hook entry for Claude Code and Codex: hook JSON on stdin, exit 2 + stderr to block.
/// Policy path: first argument, else `DASHCREW_GUARD_POLICY`, else `<hook cwd>/.ai/guard.json`.
/// Any read or parse failure allows the call; `dashcrew doctor` reports a broken install.
Future<void> main(List<String> arguments) async {
  try {
    final input = jsonDecode(await stdin.transform(utf8.decoder).join());
    if (input is! Map<String, Object?>) exit(0);

    final policyPath = arguments.isNotEmpty
        ? arguments.first
        : Platform.environment['DASHCREW_GUARD_POLICY'] ??
            '${input['cwd'] as String? ?? Directory.current.path}/.ai/guard.json';
    final policyFile = File(policyPath);
    if (!policyFile.existsSync()) exit(0);
    final policy = GuardPolicy.fromJson(jsonDecode(policyFile.readAsStringSync()) as Map<String, Object?>);

    final decision = evaluateAll(GuardEvent.allFromHook(input), policy);
    if (decision.blocked) {
      stderr.writeln('[dashcrew guard] ${decision.reason}');
      exit(2);
    }
  } catch (_) {
    exit(0);
  }
  exit(0);
}
