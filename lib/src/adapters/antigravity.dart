import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/project_files.dart';
import 'adapter.dart';

/// Antigravity layout: `.agents/skills/<name>/SKILL.md` (shared with Codex), `.agents/agents/<role>.md`,
/// the guard under its own key in `.agents/hooks.json` and MCP servers in `.agents/mcp_config.json`.
/// Antigravity reads AGENTS.md itself; it cuts every rule file at 24,000 bytes.
class AntigravityAdapter extends Adapter {
  const AntigravityAdapter(this.catalog, {required this.projectDir});

  final Catalog catalog;
  final String projectDir;

  @override
  String get agent => 'antigravity';

  /// Tool names from the Antigravity hooks reference; a subagent with an unknown name may hang.
  static const knownTools = {
    'view_file',
    'write_to_file',
    'replace_file_content',
    'multi_replace_file_content',
    'list_dir',
    'find_by_name',
    'grep_search',
    'search_web',
    'read_url_content',
    'run_command',
    'manage_task',
    'schedule',
    'list_permissions',
    'ask_permission',
    'invoke_subagent',
    'define_subagent',
    'send_message',
    'manage_subagents',
    'ask_question',
    'generate_image',
  };

  /// Key dashcrew owns in `.agents/hooks.json`; every other key belongs to the user or another tool.
  static const guardKey = 'dashcrew-guard';

  /// Antigravity hooks name the tool in `toolCall.name`; the command has no documented shell or working directory,
  /// so it is a plain relative path like the docs' own example.
  static const guardHook = <String, Object?>{
    'PreToolUse': [
      {
        'matcher': '^(run_command|write_to_file|replace_file_content|multi_replace_file_content)\$',
        'hooks': [
          {'type': 'command', 'command': './.ai/bin/guard', 'timeout': 30},
        ],
      },
    ],
  };

  /// MCP servers dashcrew keeps in `.agents/mcp_config.json`.
  static const mcpServers = <String, Map<String, Object?>>{
    'dart-mcp-server': {
      'command': 'dart',
      'args': ['mcp-server'],
    },
  };

  static const _readTools = ['view_file', 'list_dir', 'find_by_name', 'grep_search', 'run_command'];

  /// The `tools` allowlist per role capability: Antigravity enforces it, so a `read` role cannot edit.
  /// `write-plans` cannot be limited to the plans folder here; that limit stays an instruction.
  static const toolsByCapability = <String, List<String>>{
    'read': _readTools,
    'write-plans': [..._readTools, 'write_to_file'],
    'write-code': [..._readTools, 'write_to_file', 'replace_file_content', 'multi_replace_file_content'],
  };

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
      files['.agents/agents/$name.md'] = _role(role, target.content);
    }
    for (final doc in skillDocs(catalog, profile, targets, warnings)) {
      files['.agents/skills/${doc.name}/SKILL.md'] = agentsSkillFile(doc.name, doc.meta, doc.body);
    }
    return files;
  }

  @override
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings) {
    final written = <String>[];
    if (_mergeHooks()) written.add('.agents/hooks.json');
    final mcp = mergeJsonMcpServers(ProjectFiles(projectDir), '.agents/mcp_config.json', mcpServers, ownedMcp, warnings);
    if (mcp.changed) written.add('.agents/mcp_config.json');
    return (written: written, ownedMcp: mcp.owned);
  }

  bool _mergeHooks() {
    final project = ProjectFiles(projectDir);
    final file = project.file('.agents/hooks.json');
    final config = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
    config[guardKey] = guardHook;
    return project.writeIfChanged('.agents/hooks.json', '${const JsonEncoder.withIndent('  ').convert(config)}\n');
  }

  String _role(RoleMeta role, String body) => '---\n'
      'name: ${role.name}\n'
      'description: ${jsonEncode(role.description)}\n'
      'model: inherit\n'
      'tools: [${toolsByCapability[role.capability]!.join(', ')}]\n'
      '---\n\n'
      '$body';
}
