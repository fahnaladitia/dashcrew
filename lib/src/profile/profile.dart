import 'package:yaml/yaml.dart';

import '../yaml_util.dart';

/// Error in `.ai/project.yaml`, with the YAML path that caused it.
class ProfileException implements Exception {
  ProfileException(this.path, this.message);

  final String path;
  final String message;

  @override
  String toString() => 'project.yaml $path: $message';
}

/// One `stack.<family>` choice: the pack option and its params.
class StackChoice {
  const StackChoice(this.option, this.params);

  final String option;
  final Map<String, Object?> params;
}

/// Parsed `.ai/project.yaml` with defaults applied.
/// `stack` options are validated against the catalog passed as [Profile.parse] `knownOptions`.
class Profile {
  Profile._({
    required this.agents,
    required this.language,
    required this.stack,
    required this.project,
    required this.policy,
    required this.paths,
    required this.ci,
  });

  static const supportedAgents = {'claude-code', 'codex', 'antigravity'};

  /// Option valid for every family: a library with no pack, named by the required `name` param.
  static const otherOption = 'other';

  static const _topLevelKeys = {'toolkit', 'agents', 'language', 'stack', 'project', 'policy', 'paths', 'ci'};

  static const _ciDefaults = <String, Object?>{'github': false, 'dashcrew_ref': null};

  static const _languageDefaults = <String, Object?>{'dartdoc': 'en', 'pr': 'en', 'chat': 'en'};

  static const _policyDefaults = <String, Object?>{
    'agent_commit': false,
    'formatter': false,
    'dartdoc_max_lines': 3,
    'tier_gates': true,
    'terse_prompts': false,
    'plan_gate': false,
  };

  static const _pathDefaults = <String, Object?>{
    'plans': '.ai/plans',
    'audits': '.ai/audits',
    'pr': '.ai/pr',
    'memory': '.ai/memory',
  };

  static const _projectDefaults = <String, Object?>{
    'flavors': <String, Object?>{},
    'local_packages': <Object?>[],
    'base_branches': ['main'],
    'branch_prefix': {'feat': 'feat/', 'fix': 'fix/', 'hotfix': 'hotfix/'},
    'build_wrapper': null,
  };

  final List<String> agents;
  final Map<String, Object?> language;
  final Map<String, StackChoice> stack;
  final Map<String, Object?> project;
  final Map<String, Object?> policy;
  final Map<String, Object?> paths;

  /// `github: true` makes sync write a workflow running `dashcrew check` at the pinned `dashcrew_ref`.
  final Map<String, Object?> ci;

  static Profile parse(String yaml, {required Map<String, Set<String>> knownOptions}) {
    final root = plainYaml(loadYaml(yaml));
    if (root is! Map<String, Object?>) throw ProfileException('', 'expected a YAML map');

    for (final key in root.keys) {
      if (!_topLevelKeys.contains(key)) throw ProfileException(key, 'unknown key');
    }
    if (root['toolkit'] != 1) throw ProfileException('toolkit', 'required, supported version is 1');

    return Profile._(
      agents: _agents(root['agents']),
      language: _merge('language', _languageDefaults, root['language'], strict: true),
      stack: _stack(root['stack'], knownOptions),
      project: _project(root['project']),
      policy: _merge('policy', _policyDefaults, root['policy'], strict: true),
      paths: _merge('paths', _pathDefaults, root['paths'], strict: false),
      ci: _ci(root['ci']),
    );
  }

  static List<String> _agents(Object? value) {
    if (value is! List || value.isEmpty) throw ProfileException('agents', 'required, a non-empty list');
    for (final agent in value) {
      if (!supportedAgents.contains(agent)) {
        throw ProfileException('agents', 'unsupported agent "$agent", expected one of ${supportedAgents.join(', ')}');
      }
    }
    return value.cast<String>();
  }

  static Map<String, Object?> _merge(String path, Map<String, Object?> defaults, Object? value, {required bool strict}) {
    if (value == null) return Map.of(defaults);
    if (value is! Map<String, Object?>) throw ProfileException(path, 'expected a map');
    if (strict) {
      for (final key in value.keys) {
        if (!defaults.containsKey(key)) throw ProfileException('$path.$key', 'unknown key');
      }
    }
    return {...defaults, ...value};
  }

  static Map<String, Object?> _project(Object? value) {
    final project = _merge('project', _projectDefaults, value, strict: false);
    final flavors = project['flavors'];
    if (flavors is! Map<String, Object?>) throw ProfileException('project.flavors', 'expected a map of flavor name to entrypoint');
    for (final MapEntry(key: name, value: entrypoint) in flavors.entries) {
      if (entrypoint is! String || entrypoint.isEmpty) {
        throw ProfileException('project.flavors.$name', 'expected the entrypoint path, e.g. lib/main_$name.dart');
      }
    }
    return project;
  }

  static Map<String, Object?> _ci(Object? value) {
    final ci = _merge('ci', _ciDefaults, value, strict: true);
    final ref = ci['dashcrew_ref'];
    if (ci['github'] == true && (ref is! String || ref.isEmpty)) {
      throw ProfileException('ci.dashcrew_ref', 'required with github: true, the dashcrew commit or tag CI installs');
    }
    return ci;
  }

  static Map<String, StackChoice> _stack(Object? value, Map<String, Set<String>> knownOptions) {
    if (value == null) return {};
    if (value is! Map<String, Object?>) throw ProfileException('stack', 'expected a map');

    final choices = <String, StackChoice>{};
    for (final MapEntry(key: family, value: raw) in value.entries) {
      final options = knownOptions[family];
      if (options == null) throw ProfileException('stack.$family', 'unknown stack family');

      final String? option;
      final params = <String, Object?>{};
      if (raw is String) {
        option = raw;
      } else if (raw is Map<String, Object?>) {
        option = raw['use'] as String?;
        params.addAll(raw);
        params.remove('use');
      } else {
        option = null;
      }
      if (option == null || !(options.contains(option) || option == otherOption)) {
        throw ProfileException('stack.$family', 'expected one of ${{...options, otherOption}.join(', ')}');
      }
      final name = params['name'];
      if (option == otherOption && (name is! String || name.isEmpty)) {
        throw ProfileException('stack.$family.name', 'required with use: $otherOption, the library package name');
      }
      choices[family] = StackChoice(option, params);
    }
    return choices;
  }

  /// Template context: every section as plain maps; `stack.<family>` is `{use: option, ...params}`.
  /// `stack_other` lists the `other` choices as `{family, name}` sorted by family.
  /// `build.flavors` lists `project.flavors` as `{name, entrypoint}` sorted by name.
  Map<String, Object?> toContext() {
    final flavors = project['flavors'] as Map<String, Object?>;
    final buildFlavors = [
      for (final name in flavors.keys.toList()..sort()) {'name': name, 'entrypoint': flavors[name]},
    ];
    final others = [
      for (final family in stack.keys.toList()..sort())
        if (stack[family]!.option == otherOption) {'family': family, 'name': stack[family]!.params['name']},
    ];
    return {
      'agents': agents,
      'language': language,
      'stack': {
        for (final MapEntry(key: family, value: choice) in stack.entries)
          family: {'use': choice.option, ...choice.params},
      },
      'stack_other': others,
      'has_stack_other': others.isNotEmpty,
      'project': project,
      'policy': policy,
      'paths': paths,
      'ci': ci,
      'build': {'flavors': buildFlavors, 'has_flavors': buildFlavors.isNotEmpty},
    };
  }
}
