import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/project_files.dart';
import 'adapter.dart';

/// Codex layout: `.agents/skills/<name>/SKILL.md` for skills and workflows, `.codex/agents/<role>.toml` for roles,
/// the guard hook in `.codex/hooks.json` and MCP servers in a `.codex/config.toml` block.
/// Codex reads AGENTS.md itself. Project `.codex/` files load only once the user trusts the project and its hooks.
class CodexAdapter extends Adapter {
  const CodexAdapter(this.catalog, {required this.projectDir});

  final Catalog catalog;
  final String projectDir;

  @override
  String get agent => 'codex';

  static const guardMatcher = '^(Bash|apply_patch|Edit|Write)\$';
  static const guardCommand =
      r'"$(git rev-parse --show-toplevel)/.ai/bin/guard" "$(git rev-parse --show-toplevel)/.ai/guard.json"';

  /// PreToolUse entry `sync` keeps in `.codex/hooks.json`.
  static const guardHook = <String, Object?>{
    'matcher': guardMatcher,
    'hooks': [
      {'type': 'command', 'command': guardCommand, 'timeout': 30},
    ],
  };

  /// MCP servers dashcrew keeps in its `.codex/config.toml` block.
  static const mcpServers = <String, ({String command, List<String> args})>{
    'dart-mcp-server': (command: 'dart', args: ['mcp-server']),
  };
  static const _blockBegin = '# dashcrew:begin';
  static const _blockEnd = '# dashcrew:end';

  /// Characters Codex gives the initial skill list when it does not know the context window.
  static const skillListBudget = 8000;

  @override
  Map<String, String> files(
    Profile profile,
    List<RenderedTarget> targets,
    Map<String, Object?> context, {
    required Map<String, String> lockedFiles,
    List<String>? warnings,
  }) {
    final files = <String, String>{};
    for (final target in targets.where((t) => t.target.startsWith('role/'))) {
      final name = target.target.substring('role/'.length);
      final role = catalog.roles[name];
      if (role == null || !Renderer.whenMatches(role.when, context)) continue;
      files['.codex/agents/$name.toml'] = _role(role, target.content);
    }
    for (final doc in skillDocs(catalog, profile, targets, warnings)) {
      files['.agents/skills/${doc.name}/SKILL.md'] = agentsSkillFile(doc.name, doc.meta, doc.body);
    }
    return files;
  }

  @override
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings) {
    final written = <String>[];
    if (_mergeHooks()) written.add('.codex/hooks.json');
    final mcp = _mergeMcp(warnings);
    if (mcp.changed) written.add('.codex/config.toml');
    return (written: written, ownedMcp: mcp.owned);
  }

  /// Rewrites only the text between `# dashcrew:begin` and `# dashcrew:end`, kept at the end of the file:
  /// the block holds tables only, and TOML needs top-level keys before the first table.
  ({bool changed, Set<String> owned}) _mergeMcp(List<String> warnings) {
    final project = ProjectFiles(projectDir);
    final file = project.file('.codex/config.toml');
    final current = file.existsSync() ? file.readAsStringSync() : '';

    var before = current;
    var after = '';
    final begin = RegExp('^${RegExp.escape(_blockBegin)}', multiLine: true).firstMatch(current);
    final end = begin == null ? null : RegExp('^${RegExp.escape(_blockEnd)}.*\n?', multiLine: true).firstMatch(current.substring(begin.start));
    if (begin != null && end != null) {
      before = current.substring(0, begin.start);
      after = current.substring(begin.start + end.end);
    }

    final owned = <String>{};
    final tables = StringBuffer();
    mcpServers.forEach((name, server) {
      if (definesMcpServer('$before\n$after', name)) {
        warnings.add('.codex/config.toml already defines "$name"; left as is.');
        return;
      }
      owned.add(name);
      tables
        ..writeln('[mcp_servers.$name]')
        ..writeln('command = ${jsonEncode(server.command)}')
        ..writeln('args = ${jsonEncode(server.args)}');
    });
    final block = owned.isEmpty ? '' : '$_blockBegin - written by dashcrew sync; edit outside this block\n$tables$_blockEnd\n';

    final String next;
    if (begin != null && end != null) {
      next = '$before$block$after';
    } else if (block.isEmpty) {
      next = current;
    } else {
      final separator = current.isEmpty ? '' : (current.endsWith('\n') ? '\n' : '\n\n');
      next = '$current$separator$block';
    }
    if (next == current) return (changed: false, owned: owned);
    return (changed: project.writeIfChanged('.codex/config.toml', next), owned: owned);
  }

  /// True when [toml] defines MCP server [name] as a table, dotted key or inline table.
  static bool definesMcpServer(String toml, String name) {
    final key = '"?${RegExp.escape(name)}"?';
    return RegExp('^\\s*\\[\\s*mcp_servers\\s*\\.\\s*$key\\s*\\]', multiLine: true).hasMatch(toml) ||
        RegExp('^\\s*mcp_servers\\s*\\.\\s*$key\\s*[.=]', multiLine: true).hasMatch(toml) ||
        RegExp('^\\s*$key\\s*=\\s*\\{', multiLine: true).hasMatch(toml);
  }

  bool _mergeHooks() {
    final project = ProjectFiles(projectDir);
    final file = project.file('.codex/hooks.json');
    final config = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
    final hooks = (config['hooks'] as Map?)?.cast<String, Object?>() ?? <String, Object?>{};
    final preToolUse = [...(hooks['PreToolUse'] as List?) ?? const []];

    final index = preToolUse.indexWhere((entry) => jsonEncode(entry).contains('.ai/bin/guard'));
    if (index >= 0) {
      preToolUse[index] = guardHook;
    } else {
      preToolUse.add(guardHook);
    }
    hooks['PreToolUse'] = preToolUse;
    config['hooks'] = hooks;
    return project.writeIfChanged('.codex/hooks.json', '${const JsonEncoder.withIndent('  ').convert(config)}\n');
  }

  /// Size of the skill list Codex builds from these SKILL.md files: name plus description of each.
  static int skillListChars(Iterable<String> skillFiles) {
    var total = 0;
    for (final content in skillFiles) {
      final name = RegExp(r'^name: (.*)$', multiLine: true).firstMatch(content)?.group(1) ?? '';
      final description = RegExp(r'^description: (.*)$', multiLine: true).firstMatch(content)?.group(1) ?? '';
      total += name.length + (description.startsWith('"') ? jsonDecode(description) as String : description).length;
    }
    return total;
  }

  /// Role file Codex reads from `.codex/agents/`; the role name is taken from `name`, not from the file name.
  String _role(RoleMeta role, String body) {
    final toml = StringBuffer()
      ..writeln('name = ${jsonEncode(role.name)}')
      ..writeln('description = ${jsonEncode(role.description)}');
    if (role.effort != null) toml.writeln('model_reasoning_effort = ${jsonEncode(role.effort)}');
    toml.writeln('developer_instructions = ${tomlMultiline(body)}');
    return toml.toString();
  }

  /// [text] as a TOML multi-line basic string; backslashes and `"""` are escaped.
  static String tomlMultiline(String text) {
    final escaped = text.replaceAll(r'\', r'\\').replaceAll('"""', r'""\"');
    return '"""\n$escaped"""';
  }
}
