import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import 'detect.dart';

typedef _Choice = ({String option, String? name, Map<String, Object?> params});

_Choice _choiceOf(StackCandidate candidate) => (option: candidate.option, name: candidate.name, params: candidate.params);

/// Writes `.ai/project.yaml` from what [detect] finds, confirmed per family unless `yes`.
/// Writes nothing else; `dashcrew sync` renders the toolkit afterwards.
class Init {
  const Init({required this.catalog, required this.projectDir, this.agents = const ['claude-code']});

  final Catalog catalog;
  final String projectDir;
  final List<String> agents;

  static const profilePath = '.ai/project.yaml';
  static final _packageName = RegExp(r'^[a-z][a-z0-9_]*$');

  /// [ask] returns the typed line, or null at end of input (keeps the proposal).
  /// Throws [InitException] when the profile exists and [force] is false.
  void run({
    required bool yes,
    required bool force,
    required List<String> remoteBranches,
    required String? Function(String prompt) ask,
    required void Function(String line) out,
  }) {
    final file = File(p.join(projectDir, profilePath));
    if (file.existsSync() && !force) throw InitException('$profilePath already exists. Rerun with --force to replace it.');

    final Catalog catalog;
    try {
      catalog = this.catalog.withProjectPacks(projectDir);
    } on CatalogException catch (error) {
      throw InitException('Project pack: $error');
    }
    final known = catalog.knownOptions();
    final detection = detect(projectDir, remoteBranches: remoteBranches);
    final candidates = {
      for (final MapEntry(key: family, value: list) in detection.stack.entries)
        if (known.containsKey(family)) family: [for (final candidate in list) _fit(candidate, known[family]!)],
    };

    out('Detected in $projectDir:');
    for (final family in candidates.keys.toList()..sort()) {
      out('  $family: ${candidates[family]!.map((c) => '${c.label} (${c.evidence.join(', ')})').join(' | ')}');
    }
    if (detection.flavors.isNotEmpty) out('  flavors: ${detection.flavors.keys.join(', ')} (lib/main_<flavor>.dart)');
    if (detection.localPackages.isNotEmpty) out('  local packages: ${detection.localPackages.join(', ')} (path dependencies)');
    if (detection.baseBranches.isNotEmpty) out('  base branches: ${detection.baseBranches.join(', ')} (remote branches)');

    final choices = <String, _Choice>{};
    for (final family in known.keys.toList()..sort()) {
      final proposed = candidates[family] ?? const <StackCandidate>[];
      final _Choice? choice;
      if (yes) {
        if (proposed.length > 1) {
          out('$family: skipped, ${proposed.length} candidates: ${proposed.map((c) => c.label).join(', ')}; set it in $profilePath');
        }
        choice = proposed.length == 1 ? _choiceOf(proposed.single) : null;
      } else {
        choice = _askFamily(family, known[family]!, proposed, ask, out);
      }
      if (choice != null) choices[family] = choice;
    }

    final yaml = _profileYaml(choices, detection);
    Profile.parse(yaml, knownOptions: known);
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(yaml);
    out('wrote $profilePath');
    out('next: dashcrew sync');
  }

  /// A detected option the catalog has no pack for becomes `other` with the package name.
  StackCandidate _fit(StackCandidate candidate, Set<String> options) {
    if (candidate.option == Profile.otherOption || options.contains(candidate.option)) return candidate;
    return StackCandidate(candidate.family, Profile.otherOption, candidate.packages.first)
      ..packages.addAll(candidate.packages)
      ..params.addAll(candidate.params)
      ..evidence.addAll(candidate.evidence);
  }

  _Choice? _askFamily(
    String family,
    Set<String> options,
    List<StackCandidate> proposed,
    String? Function(String prompt) ask,
    void Function(String line) out,
  ) {
    final fallback = proposed.length == 1 ? _choiceOf(proposed.single) : null;
    final shown = proposed.isEmpty ? '-' : proposed.map((c) => c.label).join(' | ');
    while (true) {
      final answer = ask('$family [$shown]: ')?.trim();
      if (answer == null || answer.isEmpty) return fallback;
      if (answer == '-') return null;
      if (options.contains(answer)) {
        final detected = proposed.where((c) => c.option == answer && c.name == null).firstOrNull;
        return detected == null ? (option: answer, name: null, params: const {}) : _choiceOf(detected);
      }
      final words = answer.split(RegExp(r'\s+'));
      if (words.length == 2 && words.first == Profile.otherOption && _packageName.hasMatch(words.last)) {
        return (option: Profile.otherOption, name: words.last, params: const {});
      }
      out('  expected one of ${[...options.toList()..sort(), '${Profile.otherOption} <package>', '- (none)'].join(', ')}');
    }
  }

  String _profileYaml(Map<String, _Choice> choices, Detection detection) {
    final yaml = StringBuffer()
      ..writeln('# Written by dashcrew init. Edit it, then run dashcrew sync.')
      ..writeln('toolkit: 1')
      ..writeln('agents: [${agents.map(_scalar).join(', ')}]');
    if (choices.isNotEmpty) {
      yaml.writeln('stack:');
      for (final family in choices.keys.toList()..sort()) {
        final choice = choices[family]!;
        final fields = [
          if (choice.name != null) 'name: ${_scalar(choice.name!)}',
          for (final MapEntry(:key, :value) in choice.params.entries) '$key: ${value is String ? _scalar(value) : value}',
        ];
        final value = fields.isEmpty ? _scalar(choice.option) : '{ use: ${_scalar(choice.option)}, ${fields.join(', ')} }';
        yaml.writeln('  $family: $value');
      }
    }
    final project = StringBuffer();
    if (detection.flavors.isNotEmpty) {
      project.writeln('  flavors:');
      detection.flavors.forEach((name, entrypoint) => project.writeln('    ${_scalar(name)}: ${_scalar(entrypoint)}'));
    }
    if (detection.localPackages.isNotEmpty) {
      project.writeln('  local_packages: [${detection.localPackages.map(_scalar).join(', ')}]');
    }
    if (detection.baseBranches.isNotEmpty) {
      project.writeln('  base_branches: [${detection.baseBranches.map(_scalar).join(', ')}]');
    }
    if (project.isNotEmpty) yaml.write('project:\n$project');
    return yaml.toString();
  }

  /// Plain YAML when safe, a JSON string (valid YAML) otherwise.
  static String _scalar(String value) {
    const reserved = {'true', 'false', 'yes', 'no', 'on', 'off', 'null', 'y', 'n'};
    final plain = RegExp(r'^[A-Za-z0-9_][A-Za-z0-9_./-]*$').hasMatch(value) && !reserved.contains(value.toLowerCase());
    return plain ? value : jsonEncode(value);
  }
}
