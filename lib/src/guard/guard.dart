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
      final patch = (input['input'] ?? input['patch'] ?? input['command'])?.toString() ?? '';
      final path = RegExp(r'^\*\*\* (?:Update|Add) File: (.+)$', multiLine: true).firstMatch(patch)?.group(1);
      return GuardEvent(kind: ToolKind.edit, filePath: path, content: patch, role: role);
    }
    return GuardEvent(kind: ToolKind.other, role: role);
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
  });

  factory GuardPolicy.fromJson(Map<String, Object?> json) => GuardPolicy(
        blockedGit: {for (final s in json['blocked_git'] as List) s as String},
        blockFormatters: json['block_formatters'] as bool,
        plansDir: json['plans_dir'] as String,
        roleCapability: (json['role_capability'] as Map).cast<String, String>(),
        forbiddenImports: (json['forbidden_imports'] as List).cast<String>(),
        localPackages: (json['local_packages'] as List).cast<String>(),
      );

  static const schema = 1;
  static const alwaysBlockedGit = {'push', 'stash', 'reset', 'rebase', 'cherry-pick', 'revert', 'merge', 'tag'};

  final Set<String> blockedGit;
  final bool blockFormatters;
  final String plansDir;
  final Map<String, String> roleCapability;
  final List<String> forbiddenImports;
  final List<String> localPackages;

  Map<String, Object?> toJson() => {
        'schema': schema,
        'blocked_git': blockedGit.toList()..sort(),
        'block_formatters': blockFormatters,
        'plans_dir': plansDir,
        'role_capability': Map.fromEntries(roleCapability.entries.toList()..sort((a, b) => a.key.compareTo(b.key))),
        'forbidden_imports': forbiddenImports,
        'local_packages': localPackages,
      };
}

const _wrappers = {'sudo', 'env', 'command', 'nohup', 'time', 'rtk', 'fvm'};
const _gitOptionsWithValue = {'-C', '-c', '--git-dir', '--work-tree', '--namespace', '--exec-path', '--config-env'};
final _assignment = RegExp(r'^[A-Za-z_][A-Za-z0-9_]*=');

/// Allows or blocks one tool call under [policy].
GuardDecision evaluate(GuardEvent event, GuardPolicy policy) {
  switch (event.kind) {
    case ToolKind.shell:
      return _evaluateShell(event.command ?? '', policy);
    case ToolKind.write:
    case ToolKind.edit:
      return _evaluateWrite(event, policy);
    case ToolKind.other:
      return const GuardDecision.allow();
  }
}

GuardDecision _evaluateShell(String script, GuardPolicy policy) {
  for (final raw in shellCommands(script)) {
    final argv = _stripPrefixes(raw);
    if (argv.isEmpty) continue;
    final program = argv.first.split('/').last;

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

GuardDecision _evaluateWrite(GuardEvent event, GuardPolicy policy) {
  final path = (event.filePath ?? '').replaceAll(r'\', '/');
  final capability = policy.roleCapability[event.role];

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

bool _isAppLib(String path, List<String> localPackages) {
  if (!path.contains('/lib/') && !path.startsWith('lib/')) return false;
  for (final package in localPackages) {
    if (path.contains('/$package/lib/') || path.startsWith('$package/lib/')) return false;
  }
  return true;
}
