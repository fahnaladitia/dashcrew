import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import 'detect.dart';

typedef _Choice = ({String option, String? name, Map<String, Object?> params});

_Choice _choiceOf(StackCandidate candidate) => (option: candidate.option, name: candidate.name, params: candidate.params);

/// Writes `.ai/project.yaml` from what [detectPlatforms] and [detect] find, confirmed per question unless `yes`.
/// A project that matches no platform is offered `platform: other`, never the default platform.
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
    final matches = detectPlatforms(projectDir, catalog.platforms);
    out('Detected in $projectDir:');
    for (final name in matches) {
      final files = catalog.platforms[name]!.detect.where((file) => File(p.join(projectDir, file)).existsSync());
      out('  platform: $name (${files.join(', ')})');
    }
    if (matches.isEmpty) {
      final expected = [for (final name in catalog.platforms.keys.toList()..sort()) '$name (${catalog.platforms[name]!.detect.join(', ')})'];
      out('  platform: none of ${expected.join(', ')}');
    }
    final platform = yes ? _platformWithoutAsking(matches) : _askPlatform(catalog, matches, ask, out);
    final platformConfig = platform == Profile.otherOption ? _askPlatformConfig(ask, out) : null;

    final known = catalog.knownOptions()[platform]!;
    final detection = detect(projectDir, remoteBranches: remoteBranches, platform: platform);
    final candidates = {
      for (final MapEntry(key: family, value: list) in detection.stack.entries)
        if (known.containsKey(family)) family: [for (final candidate in list) _fit(candidate, known[family]!)],
    };

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

    final yaml = _profileYaml(platform, platformConfig, choices, detection);
    Profile.parse(yaml, knownOptions: catalog.knownOptions());
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(yaml);
    out('wrote $profilePath');
    out('next: dashcrew sync');
  }

  String _platformWithoutAsking(List<String> matches) {
    if (matches.length == 1) return matches.single;
    if (matches.isEmpty) {
      throw InitException('No platform detected. Rerun without --yes to declare platform: ${Profile.otherOption} and its commands.');
    }
    throw InitException('${matches.length} platforms detected: ${matches.join(', ')}. Rerun without --yes and pick one.');
  }

  /// One detected platform is proposed; none proposes `other`; several propose nothing and need an answer.
  String _askPlatform(Catalog catalog, List<String> matches, String? Function(String prompt) ask, void Function(String line) out) {
    final options = [...catalog.platforms.keys.toList()..sort(), Profile.otherOption];
    final proposal = matches.isEmpty ? Profile.otherOption : (matches.length == 1 ? matches.single : null);
    final shown = matches.isEmpty ? Profile.otherOption : matches.join(' | ');
    while (true) {
      final answer = ask('platform [$shown]: ')?.trim();
      if (answer == null && proposal == null) throw InitException('Several platforms detected (${matches.join(', ')}); pick one.');
      if (answer == null || answer.isEmpty) {
        if (proposal != null) return proposal;
      } else if (options.contains(answer)) {
        return answer;
      }
      out('  expected one of ${options.join(', ')}');
    }
  }

  /// `platform_config` for `other`: the detect files and each command, asked one by one.
  Map<String, Object?> _askPlatformConfig(String? Function(String prompt) ask, void Function(String line) out) {
    String? answer(String prompt, {required bool required}) {
      while (true) {
        final value = ask(prompt)?.trim();
        if (value == null) {
          if (!required) return null;
          throw InitException('platform: ${Profile.otherOption} needs ${prompt.replaceAll(RegExp(r'[:\s]+$'), '')}; '
              'rerun dashcrew init, or write platform_config in $profilePath by hand.');
        }
        if (value.isNotEmpty) return value;
        if (!required) return null;
        out('  required');
      }
    }

    final detect = answer('detect (files that identify the platform, space separated): ', required: true)!;
    return {
      'detect': detect.split(RegExp(r'\s+')),
      'commands': {
        for (final key in Profile.platformCommands) key: answer('commands.$key: ', required: true),
        for (final key in Profile.optionalPlatformCommands) key: answer('commands.$key (empty for none): ', required: false),
      },
    };
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

  String _profileYaml(String platform, Map<String, Object?>? platformConfig, Map<String, _Choice> choices, Detection detection) {
    final yaml = StringBuffer()
      ..writeln('# Written by dashcrew init. Edit it, then run dashcrew sync.')
      ..writeln('toolkit: 1')
      ..writeln('platform: ${_scalar(platform)}');
    if (platformConfig != null) {
      yaml
        ..writeln('platform_config:')
        ..writeln('  detect: [${(platformConfig['detect'] as List<String>).map(_scalar).join(', ')}]')
        ..writeln('  commands:');
      (platformConfig['commands'] as Map<String, String?>).forEach((key, command) {
        if (command != null) yaml.writeln('    $key: ${_scalar(command)}');
      });
    }
    yaml.writeln('agents: [${agents.map(_scalar).join(', ')}]');
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
