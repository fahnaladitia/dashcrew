import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/project_files.dart';
import 'adapter.dart';

/// Codex layout: `.agents/skills/<name>/SKILL.md` for skills and workflows; the guard hook in `.codex/hooks.json`.
/// Codex reads AGENTS.md itself. Project `.codex/` files load only once the user trusts the project and its hooks.
class CodexAdapter extends Adapter {
  const CodexAdapter(this.catalog, {required this.projectDir});

  final Catalog catalog;
  final String projectDir;

  @override
  String get agent => 'codex';

  static const guardMatcher = '^(Bash|apply_patch|Edit|Write)\$';
  static const guardCommand =
      r'"\$(git rev-parse --show-toplevel)/.ai/bin/guard" "\$(git rev-parse --show-toplevel)/.ai/guard.json"';

  /// PreToolUse entry `sync` keeps in `.codex/hooks.json`.
  static const guardHook = <String, Object?>{
    'matcher': guardMatcher,
    'hooks': [
      {'type': 'command', 'command': guardCommand, 'timeout': 30},
    ],
  };

  /// Characters Codex gives the initial skill list when it does not know the context window.
  static const skillListBudget = 8000;

  @override
  Map<String, String> files(
    Profile profile,
    List<RenderedTarget> targets,
    Map<String, Object?> context, {
    required Map<String, String> lockedFiles,
    List<String>? warnings,
  }) =>
      {
        for (final doc in skillDocs(catalog, profile, targets, warnings))
          '.agents/skills/${doc.name}/SKILL.md': _skill(doc.name, doc.meta, doc.body),
      };

  @override
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings) {
    final written = <String>[];
    if (_mergeHooks()) written.add('.codex/hooks.json');
    return (written: written, ownedMcp: ownedMcp);
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

  /// Codex front matter holds `name` and `description` only; the trigger and argument hint move into them.
  String _skill(String name, DocMeta meta, String body) {
    final description = meta.whenToUse == null ? meta.description : '${meta.description} Use when: ${meta.whenToUse}';
    final arguments = meta.argumentHint == null ? '' : 'Arguments: `${meta.argumentHint}`\n\n';
    return '---\nname: $name\ndescription: ${jsonEncode(description)}\n---\n\n$arguments$body';
  }
}
