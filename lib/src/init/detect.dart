import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

import '../profile/profile.dart';
import '../yaml_util.dart';

/// `init` refused to run; [message] says why and how to proceed.
class InitException implements Exception {
  InitException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// Stack choice implied by one dependency; [name] is set for `other`.
class DetectionRule {
  const DetectionRule(this.family, this.option, [this.name]);

  final String family;
  final String option;
  final String? name;
}

/// Dependency name -> stack choice. Adding a library is one line here plus a test row.
const detectionRules = <String, DetectionRule>{
  'flutter_bloc': DetectionRule('state', 'bloc'),
  'bloc': DetectionRule('state', 'bloc'),
  'flutter_riverpod': DetectionRule('state', 'riverpod'),
  'hooks_riverpod': DetectionRule('state', 'riverpod'),
  'riverpod': DetectionRule('state', 'riverpod'),
  'provider': DetectionRule('state', 'provider'),
  'mobx': DetectionRule('state', Profile.otherOption, 'mobx'),
  'flutter_mobx': DetectionRule('state', Profile.otherOption, 'mobx'),
  'get_it': DetectionRule('di', 'get_it'),
  'get': DetectionRule('router', 'getx'),
  'go_router': DetectionRule('router', Profile.otherOption, 'go_router'),
  'auto_route': DetectionRule('router', Profile.otherOption, 'auto_route'),
  'dio': DetectionRule('network', 'dio'),
  'http': DetectionRule('network', Profile.otherOption, 'http'),
  'chopper': DetectionRule('network', Profile.otherOption, 'chopper'),
  'hive': DetectionRule('storage', 'hive'),
  'hive_ce': DetectionRule('storage', 'hive'),
  'hive_flutter': DetectionRule('storage', 'hive'),
  'hive_ce_flutter': DetectionRule('storage', 'hive'),
  'isar': DetectionRule('storage', Profile.otherOption, 'isar'),
  'drift': DetectionRule('storage', Profile.otherOption, 'drift'),
  'sqflite': DetectionRule('storage', Profile.otherOption, 'sqflite'),
  'posthog_flutter': DetectionRule('analytics', 'posthog'),
  'mocktail': DetectionRule('test', Profile.otherOption, 'mocktail'),
  'mockito': DetectionRule('test', Profile.otherOption, 'mockito'),
};

/// Remote branches that `init` proposes as base branches, in this order.
const baseBranchCandidates = ['main', 'master', 'develop', 'dev'];

/// One proposed value for a stack family, with the dependencies that led to it.
class StackCandidate {
  StackCandidate(this.family, this.option, this.name);

  final String family;
  final String option;
  final String? name;

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

/// Reads `pubspec.yaml`, `lib/main_<flavor>.dart` and [remoteBranches] (`origin/<name>`).
Detection detect(String projectDir, {required List<String> remoteBranches}) {
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
      final rule = detectionRules[name];
      if (rule == null) continue;
      final candidates = stack[rule.family] ??= [];
      var candidate = candidates.where((c) => c.option == rule.option && c.name == rule.name).firstOrNull;
      if (candidate == null) {
        candidate = StackCandidate(rule.family, rule.option, rule.name);
        candidates.add(candidate);
      }
      candidate
        ..packages.add(name)
        ..evidence.add('$name in $section');
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
    baseBranches: [for (final name in baseBranchCandidates) if (remoteBranches.contains('origin/$name')) name],
  );
}

/// `origin/<name>` for every remote-tracking branch; empty outside a git repository.
List<String> remoteBranches(String projectDir) {
  final result = Process.runSync('git', ['branch', '-r', '--format=%(refname:short)'], workingDirectory: projectDir);
  if (result.exitCode != 0) return const [];
  return [for (final line in (result.stdout as String).split('\n')) if (line.trim().isNotEmpty) line.trim()];
}
