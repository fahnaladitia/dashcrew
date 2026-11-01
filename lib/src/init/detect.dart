import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../yaml_util.dart';

/// `init` refused to run; [message] says why and how to proceed.
class InitException implements Exception {
  InitException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// Stack choice implied by one dependency; [name] is set for `other`, [params] for a pack setting it implies.
class DetectionRule {
  const DetectionRule(this.family, this.option, {this.name, this.params = const {}});

  final String family;
  final String option;
  final String? name;
  final Map<String, Object?> params;
}

/// Dependency name -> the stack choices it implies; one library may fill several families.
/// Adding a library is one entry here plus a test row.
const detectionRules = <String, List<DetectionRule>>{
  'flutter_bloc': [DetectionRule('state', 'bloc')],
  'bloc': [DetectionRule('state', 'bloc')],
  'flutter_riverpod': [DetectionRule('state', 'riverpod'), DetectionRule('di', 'riverpod')],
  'hooks_riverpod': [DetectionRule('state', 'riverpod'), DetectionRule('di', 'riverpod')],
  'riverpod': [DetectionRule('state', 'riverpod'), DetectionRule('di', 'riverpod')],
  'provider': [DetectionRule('state', 'provider'), DetectionRule('di', 'provider')],
  'mobx': [DetectionRule('state', Profile.otherOption, name: 'mobx')],
  'flutter_mobx': [DetectionRule('state', Profile.otherOption, name: 'mobx')],
  'get_it': [DetectionRule('di', 'get_it')],
  'injectable': [DetectionRule('di', 'get_it', params: {'injectable': true})],
  'get': [DetectionRule('router', 'getx')],
  'go_router': [DetectionRule('router', 'go_router')],
  'auto_route': [DetectionRule('router', 'auto_route')],
  'dio': [DetectionRule('network', 'dio')],
  'http': [DetectionRule('network', 'http')],
  'retrofit': [DetectionRule('network', 'retrofit')],
  'chopper': [DetectionRule('network', Profile.otherOption, name: 'chopper')],
  'hive': [DetectionRule('storage', 'hive')],
  'hive_ce': [DetectionRule('storage', 'hive')],
  'hive_flutter': [DetectionRule('storage', 'hive')],
  'hive_ce_flutter': [DetectionRule('storage', 'hive')],
  'shared_preferences': [DetectionRule('storage', 'shared_preferences')],
  'sqflite': [DetectionRule('storage', 'sqflite')],
  'isar': [DetectionRule('storage', Profile.otherOption, name: 'isar')],
  'drift': [DetectionRule('storage', Profile.otherOption, name: 'drift')],
  'posthog_flutter': [DetectionRule('analytics', 'posthog')],
  'firebase_analytics': [DetectionRule('analytics', 'firebase')],
  'mixpanel_flutter': [DetectionRule('analytics', 'mixpanel')],
  'mocktail': [DetectionRule('test', 'mocktail')],
  'mockito': [DetectionRule('test', 'mockito')],
};

/// Remote branches that `init` proposes as base branches, in this order.
const baseBranchCandidates = ['main', 'master', 'develop', 'dev'];

/// One proposed value for a stack family, with the dependencies that led to it.
class StackCandidate {
  StackCandidate(this.family, this.option, this.name);

  final String family;
  final String option;
  final String? name;

  /// Pack params the dependencies imply, such as `injectable: true` for get_it.
  final Map<String, Object?> params = {};

  /// Dependency names behind this candidate, and the same with their pubspec section.
  final List<String> packages = [];
  final List<String> evidence = [];

  /// `bloc`, or `other <name>` for a library without a pack.
  String get label => name == null ? option : '$option $name';
}

/// What `init` read from the project; it guesses nothing beyond these sources.
class Detection {
  const Detection({required this.stack, required this.flavors, required this.localPackages, required this.baseBranches});

  /// Family -> distinct candidates; more than one means the user has to choose.
  final Map<String, List<StackCandidate>> stack;
  final Map<String, String> flavors;
  final List<String> localPackages;
  final List<String> baseBranches;
}

/// Names of the [platforms] with at least one `detect` file in [projectDir], sorted.
List<String> detectPlatforms(String projectDir, Map<String, PlatformSpec> platforms) => [
      for (final name in platforms.keys.toList()..sort())
        if (platforms[name]!.detect.any((file) => File(p.join(projectDir, file)).existsSync())) name,
    ];

/// Reads [remoteBranches] (`origin/<name>`) and, for the default platform only, `pubspec.yaml` and
/// `lib/main_<flavor>.dart`; any other [platform] gets no stack, flavor or local package proposal.
Detection detect(String projectDir, {required List<String> remoteBranches, String platform = Profile.defaultPlatform}) {
  final baseBranches = [for (final name in baseBranchCandidates) if (remoteBranches.contains('origin/$name')) name];
  if (platform != Profile.defaultPlatform) {
    return Detection(stack: const {}, flavors: const {}, localPackages: const [], baseBranches: baseBranches);
  }
  final pubspecFile = File(p.join(projectDir, 'pubspec.yaml'));
  if (!pubspecFile.existsSync()) throw InitException('No pubspec.yaml in $projectDir. Run dashcrew init in the Flutter project root.');
  final pubspec = plainYaml(loadYaml(pubspecFile.readAsStringSync()));
  if (pubspec is! Map<String, Object?>) throw InitException('pubspec.yaml is not a YAML map.');

  final stack = <String, List<StackCandidate>>{};
  final localPackages = <String>[];
  for (final section in ['dependencies', 'dev_dependencies']) {
    final dependencies = pubspec[section];
    if (dependencies is! Map<String, Object?>) continue;
    for (final name in dependencies.keys.toList()..sort()) {
      final source = dependencies[name];
      if (source is Map && source['path'] is String) {
        final path = p.posix.normalize((source['path'] as String).replaceAll(r'\', '/'));
        if (!path.startsWith('..') && !p.posix.isAbsolute(path)) localPackages.add(path);
      }
      for (final rule in detectionRules[name] ?? const <DetectionRule>[]) {
        final candidates = stack[rule.family] ??= [];
        var candidate = candidates.where((c) => c.option == rule.option && c.name == rule.name).firstOrNull;
        if (candidate == null) {
          candidate = StackCandidate(rule.family, rule.option, rule.name);
          candidates.add(candidate);
        }
        candidate
          ..params.addAll(rule.params)
          ..packages.add(name)
          ..evidence.add('$name in $section');
      }
    }
  }

  final flavors = <String, String>{};
  final lib = Directory(p.join(projectDir, 'lib'));
  if (lib.existsSync()) {
    final entrypoint = RegExp(r'^main_([a-z0-9_]+)\.dart$');
    for (final file in lib.listSync().whereType<File>().toList()..sort((a, b) => a.path.compareTo(b.path))) {
      final match = entrypoint.firstMatch(p.basename(file.path));
      if (match != null) flavors[match.group(1)!] = 'lib/${p.basename(file.path)}';
    }
  }

  return Detection(
    stack: stack,
    flavors: flavors,
    localPackages: localPackages,
    baseBranches: baseBranches,
  );
}

/// `origin/<name>` for every remote-tracking branch; empty outside a git repository.
List<String> remoteBranches(String projectDir) {
  final result = Process.runSync('git', ['branch', '-r', '--format=%(refname:short)'], workingDirectory: projectDir);
  if (result.exitCode != 0) return const [];
  return [for (final line in (result.stdout as String).split('\n')) if (line.trim().isNotEmpty) line.trim()];
}
