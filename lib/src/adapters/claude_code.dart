import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/project_files.dart';
import 'adapter.dart';

/// Claude Code layout: `.claude/agents/<role>.md`, `.claude/skills/<name>/SKILL.md` (skills and workflows), `CLAUDE.md`.
/// `.claude/settings.json` and `.mcp.json` are merged, never replaced.
class ClaudeCodeAdapter extends Adapter {
  ClaudeCodeAdapter(this.catalog, {required String projectDir}) : _project = ProjectFiles(projectDir);

  final Catalog catalog;
  final ProjectFiles _project;

  @override
  String get agent => 'claude-code';

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

  /// A skill no selected pack describes is skipped with a warning (its owner was replaced by a project pack).
  /// Throws [StateError] when a rendered workflow has no description in the catalog.
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
      files['.claude/agents/$name.md'] = _agent(role, target.content);
    }
    for (final doc in skillDocs(catalog, profile, targets, warnings)) {
      files['.claude/skills/${doc.name}/SKILL.md'] = _skill(doc.name, doc.meta, doc.body);
    }

    final claudeMd = _project.file('CLAUDE.md');
    if (!claudeMd.existsSync() || lockedFiles.containsKey('CLAUDE.md')) {
      files['CLAUDE.md'] = '@AGENTS.md\n';
    } else if (!claudeMd.readAsStringSync().contains('@AGENTS.md')) {
      warnings?.add('CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md" so Claude Code loads the rules.');
    }
    return files;
  }

  @override
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings) {
    final written = <String>[];
    if (_mergeSettings()) written.add('.claude/settings.json');
    final mcp = _mergeMcp(ownedMcp, warnings);
    if (mcp.changed) written.add('.mcp.json');
    return (written: written, ownedMcp: mcp.owned);
  }

  bool _mergeSettings() {
    final file = _project.file('.claude/settings.json');
    final settings = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
    final hooks = (settings['hooks'] as Map?)?.cast<String, Object?>() ?? <String, Object?>{};
    final preToolUse = [...(hooks['PreToolUse'] as List?) ?? const []];

    final index = preToolUse.indexWhere((entry) => jsonEncode(entry).contains('.ai/bin/guard'));
    if (index >= 0) {
      preToolUse[index] = guardHook;
    } else {
      preToolUse.add(guardHook);
    }
    hooks['PreToolUse'] = preToolUse;
    settings['hooks'] = hooks;
    return _project.writeIfChanged('.claude/settings.json', '${const JsonEncoder.withIndent('  ').convert(settings)}\n');
  }

  ({bool changed, Set<String> owned}) _mergeMcp(Set<String> owned, List<String> warnings) =>
      mergeJsonMcpServers(_project, '.mcp.json', mcpServers, owned, warnings);


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
