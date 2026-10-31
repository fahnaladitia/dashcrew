import 'dart:convert';

import 'gate_state.dart';
import 'shell.dart';

/// Kind of tool call the guard sees.
enum ToolKind { shell, write, edit, other }

/// Hook input normalized across agents.
class GuardEvent {
  const GuardEvent({required this.kind, this.command, this.filePath, this.content, this.role});

  /// Claude Code: `Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `agent_type`.
  /// Codex: `shell` / `exec_command` with a string or argv list (`bash -lc <script>` is unwrapped), `apply_patch`.
  factory GuardEvent.fromHook(Map<String, Object?> json) {
    final tool = json['tool_name'] as String? ?? '';
    final input = json['tool_input'] is Map ? (json['tool_input'] as Map).cast<String, Object?>() : <String, Object?>{};
    final role = json['agent_type'] as String?;

    if (_shellTools.contains(tool)) {
      return GuardEvent(kind: ToolKind.shell, command: _shellText(input['command']), role: role);
    }
    if (tool == 'Write') {
      return GuardEvent(
        kind: ToolKind.write,
        filePath: input['file_path'] as String?,
        content: input['content'] as String?,
        role: role,
      );
    }
    if (tool == 'Edit' || tool == 'MultiEdit' || tool == 'NotebookEdit') {
      final edits = input['edits'];
      final content = edits is List
          ? edits.map((e) => (e as Map)['new_string']?.toString() ?? '').join('\n')
          : (input['new_string'] ?? input['new_source'])?.toString();
      return GuardEvent(
        kind: ToolKind.edit,
        filePath: (input['file_path'] ?? input['notebook_path']) as String?,
        content: content,
        role: role,
      );
    }
    if (tool == 'apply_patch') {
      final events = allFromHook(json);
      return events.isEmpty ? GuardEvent(kind: ToolKind.edit, role: role) : events.first;
    }
    return GuardEvent(kind: ToolKind.other, role: role);
  }

  /// Every event in one hook call: a Codex `apply_patch` gives one per file, any other tool one.
  /// A patch file's content is its added lines only; a moved file is checked at its new path.
  static List<GuardEvent> allFromHook(Map<String, Object?> json) {
    final toolCall = json['toolCall'];
    if (toolCall is Map) return _antigravity(toolCall);
    if (json['tool_name'] != 'apply_patch') return [GuardEvent.fromHook(json)];
    final input = json['tool_input'] is Map ? (json['tool_input'] as Map).cast<String, Object?>() : <String, Object?>{};
    final patch = (input['command'] ?? input['input'] ?? input['patch'])?.toString() ?? '';
    final role = json['agent_type'] as String?;
    final header = RegExp(r'^\*\*\* (Add|Update|Delete) File: (.+)$');
    final moveTo = RegExp(r'^\*\*\* Move to: (.+)$');

    final events = <GuardEvent>[];
    String? path;
    ToolKind? kind;
    final added = <String>[];
    void flush() {
      if (path != null) events.add(GuardEvent(kind: kind!, filePath: path, content: added.join('\n'), role: role));
      added.clear();
    }

    for (final line in patch.split('\n')) {
      final file = header.firstMatch(line);
      final move = moveTo.firstMatch(line);
      if (file != null) {
        flush();
        kind = file.group(1) == 'Add' ? ToolKind.write : ToolKind.edit;
        path = file.group(2)!.trim();
      } else if (move != null) {
        path = move.group(1)!.trim();
      } else if (line.startsWith('+') && path != null) {
        added.add(line.substring(1));
      }
    }
    flush();
    return events;
  }

  /// Antigravity `toolCall` input: `run_command`, `write_to_file`, `replace_file_content`, `multi_replace_file_content`.
  /// Its hooks carry no subagent identity, so the role stays null.
  static List<GuardEvent> _antigravity(Map<Object?, Object?> toolCall) {
    final args = toolCall['args'] is Map ? (toolCall['args'] as Map).cast<String, Object?>() : <String, Object?>{};
    final path = args['TargetFile'] as String?;
    switch (toolCall['name']) {
      case 'run_command':
        return [GuardEvent(kind: ToolKind.shell, command: args['CommandLine'] as String? ?? '')];
      case 'write_to_file':
        return [GuardEvent(kind: ToolKind.write, filePath: path, content: args['CodeContent'] as String?)];
      case 'replace_file_content':
        return [GuardEvent(kind: ToolKind.edit, filePath: path, content: args['ReplacementContent'] as String?)];
      case 'multi_replace_file_content':
        final chunks = args['ReplacementChunks'] is List ? args['ReplacementChunks'] as List : const [];
        final replacements = [
          for (final chunk in chunks.whereType<Map>())
            for (final MapEntry(:key, :value) in chunk.entries)
              if (key.toString().startsWith('Replacement') && value is String) value,
        ];
        return [GuardEvent(kind: ToolKind.edit, filePath: path, content: replacements.join('\n'))];
      default:
        return [const GuardEvent(kind: ToolKind.other)];
    }
  }

  static const _shellTools = {'Bash', 'shell', 'local_shell', 'exec_command', 'container.exec'};

  static String _shellText(Object? command) {
    if (command is String) return command;
    if (command is List) {
      final argv = command.map((e) => e.toString()).toList();
      final flag = argv.indexWhere((a) => a == '-c' || a == '-lc');
      if (argv.isNotEmpty && const {'bash', 'sh', 'zsh'}.contains(argv.first.split('/').last) && flag >= 0) {
        return flag + 1 < argv.length ? argv[flag + 1] : '';
      }
      return argv.join(' ');
    }
    return '';
  }

  final ToolKind kind;
  final String? command;
  final String? filePath;
  final String? content;
  final String? role;
}

/// Result of [evaluate].
class GuardDecision {
  const GuardDecision.allow()
      : blocked = false,
        reason = null;
  const GuardDecision.block(String this.reason) : blocked = true;

  final bool blocked;
  final String? reason;
}

/// Compiled guard policy, stored as `.ai/guard.json` by `sync`.
class GuardPolicy {
  const GuardPolicy({
    required this.blockedGit,
    required this.blockFormatters,
    required this.plansDir,
    required this.roleCapability,
    required this.forbiddenImports,
    required this.localPackages,
    this.planGate = false,
  });

  factory GuardPolicy.fromJson(Map<String, Object?> json) => GuardPolicy(
        blockedGit: {for (final s in json['blocked_git'] as List) s as String},
        blockFormatters: json['block_formatters'] as bool,
        plansDir: json['plans_dir'] as String,
        roleCapability: (json['role_capability'] as Map).cast<String, String>(),
        forbiddenImports: (json['forbidden_imports'] as List).cast<String>(),
        localPackages: (json['local_packages'] as List).cast<String>(),
        planGate: json['plan_gate'] as bool? ?? false,
      );

  static const schema = 1;
  static const alwaysBlockedGit = {'push', 'stash', 'reset', 'rebase', 'cherry-pick', 'revert', 'merge', 'tag'};

  final Set<String> blockedGit;
  final bool blockFormatters;
  final String plansDir;
  final Map<String, String> roleCapability;
  final List<String> forbiddenImports;
  final List<String> localPackages;

  /// App code and tests stay closed on a branch whose plan issue is not approved yet.
  final bool planGate;

  Map<String, Object?> toJson() => {
        'schema': schema,
        'blocked_git': blockedGit.toList()..sort(),
        'block_formatters': blockFormatters,
        'plans_dir': plansDir,
        'role_capability': Map.fromEntries(roleCapability.entries.toList()..sort((a, b) => a.key.compareTo(b.key))),
        'forbidden_imports': forbiddenImports,
        'local_packages': localPackages,
        'plan_gate': planGate,
      };
}

const _wrappers = {'sudo', 'env', 'command', 'nohup', 'time', 'rtk', 'fvm'};
const _gitOptionsWithValue = {'-C', '-c', '--git-dir', '--work-tree', '--namespace', '--exec-path', '--config-env'};
final _assignment = RegExp(r'^[A-Za-z_][A-Za-z0-9_]*=');

/// How the guard answers the agent that called it: Antigravity (`toolCall` input) reads a JSON decision on stdout;
/// Claude Code and Codex read exit code 2 with the reason on stderr. An allowed call prints nothing.
({int exitCode, String stdout, String stderr}) hookResponse(Map<String, Object?> input, GuardDecision decision) {
  if (!decision.blocked) return (exitCode: 0, stdout: '', stderr: '');
  final reason = '[dashcrew guard] ${decision.reason}';
  if (input['toolCall'] is Map) {
    return (exitCode: 0, stdout: jsonEncode({'decision': 'deny', 'reason': reason}), stderr: '');
  }
  return (exitCode: 2, stdout: '', stderr: reason);
}

/// Blocks when any of [events] is blocked, with the first reason.
GuardDecision evaluateAll(List<GuardEvent> events, GuardPolicy policy, {PlanGateState? gate}) {
  for (final event in events) {
    final decision = evaluate(event, policy, gate: gate);
    if (decision.blocked) return decision;
  }
  return const GuardDecision.allow();
}

/// Allows or blocks one tool call under [policy].
/// [gate] is the checked-out branch's plan gate; it matters only with [GuardPolicy.planGate].
GuardDecision evaluate(GuardEvent event, GuardPolicy policy, {PlanGateState? gate}) {
  switch (event.kind) {
    case ToolKind.shell:
      return _evaluateShell(event.command ?? '', policy);
    case ToolKind.write:
    case ToolKind.edit:
      return _evaluateWrite(event, policy, gate);
    case ToolKind.other:
      return const GuardDecision.allow();
  }
}

GuardDecision _evaluateShell(String script, GuardPolicy policy) {
  for (final raw in shellCommands(script)) {
    final argv = _stripPrefixes(raw);
    if (argv.isEmpty) continue;
    final program = argv.first.split('/').last;

    if (program == 'gh' && argv.any((word) => word.contains('/approve'))) {
      return const GuardDecision.block("Approvals are the user's, on GitHub. Run dashcrew gate status and stop.");
    }
    if (program == 'git') {
      final sub = _gitSubcommand(argv);
      if (sub != null && policy.blockedGit.contains(sub)) {
        return GuardDecision.block('git $sub is reserved for the user. Ask them to run it.');
      }
    }
    if (policy.blockFormatters && argv.length > 1) {
      final sub = argv[1];
      if ((program == 'dart' && (sub == 'format' || sub == 'fix')) || (program == 'flutter' && sub == 'format')) {
        return GuardDecision.block('$program $sub rewrites untouched lines. Fix only the lines your task needs.');
      }
    }
  }
  return const GuardDecision.allow();
}

List<String> _stripPrefixes(List<String> argv) {
  var i = 0;
  while (i < argv.length && (_assignment.hasMatch(argv[i]) || _wrappers.contains(argv[i]))) {
    i++;
  }
  return argv.sublist(i);
}

String? _gitSubcommand(List<String> argv) {
  var i = 1;
  while (i < argv.length) {
    final word = argv[i];
    if (_gitOptionsWithValue.contains(word)) {
      i += 2;
    } else if (word.startsWith('-')) {
      i++;
    } else {
      return word;
    }
  }
  return null;
}

GuardDecision _evaluateWrite(GuardEvent event, GuardPolicy policy, PlanGateState? gate) {
  final path = (event.filePath ?? '').replaceAll(r'\', '/');
  final capability = policy.roleCapability[event.role];

  if (policy.planGate && gate != null && gate.plan == 'pending' && _isAppCode(path, policy.localPackages)) {
    return GuardDecision.block('The plan for ${gate.branch} is not approved yet (issue #${gate.issue}). '
        'Wait for the user to comment /approve plan, then run dashcrew gate status.');
  }

  if (capability == 'read') {
    return GuardDecision.block('${event.role} is read-only. Return your findings instead of writing files.');
  }
  if (capability == 'write-plans') {
    if (event.kind == ToolKind.edit) {
      return GuardDecision.block('${event.role} may not edit files; write whole files under ${policy.plansDir}/ only.');
    }
    final plans = policy.plansDir.replaceAll(RegExp(r'/+$'), '');
    if (!path.contains('/$plans/') && !path.startsWith('$plans/')) {
      return GuardDecision.block('${event.role} may only write under $plans/. Source and tests belong to a write-code role.');
    }
  }

  if (path.endsWith('.dart') && _isAppLib(path, policy.localPackages)) {
    final content = event.content ?? '';
    for (final prefix in policy.forbiddenImports) {
      if (RegExp('import\\s+[\'"]${RegExp.escape(prefix)}').hasMatch(content)) {
        return GuardDecision.block('Do not import $prefix in app code; go through the project facade.');
      }
    }
  }
  return const GuardDecision.allow();
}

/// App `lib/` or `test/`, outside local packages.
bool _isAppCode(String path, List<String> localPackages) {
  if (_isAppLib(path, localPackages)) return true;
  if (!path.contains('/test/') && !path.startsWith('test/')) return false;
  return !localPackages.any((package) => path.contains('/$package/test/') || path.startsWith('$package/test/'));
}

bool _isAppLib(String path, List<String> localPackages) {
  if (!path.contains('/lib/') && !path.startsWith('lib/')) return false;
  for (final package in localPackages) {
    if (path.contains('/$package/lib/') || path.startsWith('$package/lib/')) return false;
  }
  return true;
}
