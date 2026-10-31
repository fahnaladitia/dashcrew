import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import 'adapter.dart';

/// Codex layout: `.agents/skills/<name>/SKILL.md` for skills and workflows.
/// Codex reads AGENTS.md itself, so nothing else is needed for the rules.
class CodexAdapter extends Adapter {
  const CodexAdapter(this.catalog, {required this.projectDir});

  final Catalog catalog;
  final String projectDir;

  @override
  String get agent => 'codex';

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
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings) =>
      (written: const [], ownedMcp: ownedMcp);

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
