import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';

/// Claude Code layout: `.claude/agents/<role>.md`, `.claude/skills/<name>/SKILL.md` (skills and workflows).
/// AGENTS.md, CLAUDE.md, settings and MCP are written by sync because they merge with project files.
class ClaudeCodeAdapter {
  const ClaudeCodeAdapter(this.catalog);

  final Catalog catalog;

  static const guardMatcher = 'Bash|Write|Edit|MultiEdit|NotebookEdit';
  static const guardCommand = r'"$CLAUDE_PROJECT_DIR/.ai/bin/guard" "$CLAUDE_PROJECT_DIR/.ai/guard.json"';

  /// PreToolUse entry `sync` keeps in `.claude/settings.json`.
  static const guardHook = <String, Object?>{
    'matcher': guardMatcher,
    'hooks': [
      {'type': 'command', 'command': guardCommand},
    ],
  };
  static const mcpServers = <String, Map<String, Object?>>{
    'dart-mcp-server': {
      'type': 'stdio',
      'command': 'dart',
      'args': ['mcp-server'],
    },
  };

  static const _tools = {
    'read': 'Read, Grep, Glob, Bash',
    'write-plans': 'Read, Write, Grep, Glob, Bash',
    'write-code': 'Read, Write, Edit, Grep, Glob, Bash',
  };
  static const _models = {'high': 'opus', 'standard': 'sonnet'};

  /// Relative path -> content for every agent and skill file of this profile.
  /// A skill no selected pack describes is skipped with a warning (its owner was replaced by a project pack).
  /// Throws [StateError] when a rendered workflow has no description in the catalog.
  Map<String, String> files(Profile profile, List<RenderedTarget> targets, Map<String, Object?> context,
      {List<String>? warnings}) {
    final renderer = Renderer(catalog);
    final packSkills = {for (final pack in renderer.selectedPacks(profile)) ...pack.skills};
    final files = <String, String>{};

    for (final target in targets) {
      final slash = target.target.indexOf('/');
      if (slash < 0) continue;
      final kind = target.target.substring(0, slash);
      final name = target.target.substring(slash + 1);

      switch (kind) {
        case 'role':
          final role = catalog.roles[name];
          if (role == null || !Renderer.whenMatches(role.when, context)) continue;
          files['.claude/agents/$name.md'] = _agent(role, target.content);
        case 'skill':
          final meta = catalog.skills[name] ?? packSkills[name];
          if (meta == null) {
            warnings?.add('skill $name skipped: no selected pack defines skills/$name.yaml');
            continue;
          }
          files['.claude/skills/$name/SKILL.md'] = _skill(name, meta, target.content);
        case 'workflow':
          final meta = catalog.workflows[name];
          if (meta == null) throw StateError('workflow/$name has no meta.yaml');
          files['.claude/skills/$name/SKILL.md'] = _skill(name, meta, target.content);
      }
    }
    return files;
  }

  String _agent(RoleMeta role, String body) {
    final header = StringBuffer('---\n')
      ..writeln('name: ${role.name}')
      ..writeln('description: ${jsonEncode(role.description)}')
      ..writeln('tools: ${_tools[role.capability]}');
    final model = _models[role.model];
    if (model != null) header.writeln('model: $model');
    if (role.effort != null) header.writeln('effort: ${role.effort}');
    header.writeln('---');
    return '$header\n$body';
  }

  String _skill(String name, DocMeta meta, String body) {
    final header = StringBuffer('---\n')
      ..writeln('name: $name')
      ..writeln('description: ${jsonEncode(meta.description)}');
    if (meta.whenToUse != null) header.writeln('when_to_use: ${jsonEncode(meta.whenToUse)}');
    if (meta.argumentHint != null) header.writeln('argument-hint: ${jsonEncode(meta.argumentHint)}');
    header.writeln('---');
    return '$header\n$body';
  }
}
