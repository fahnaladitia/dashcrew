import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

import '../profile/profile.dart';
import '../yaml_util.dart';

/// Error in a catalog file, with the catalog-relative path of that file.
class CatalogException implements Exception {
  CatalogException(this.source, this.message);

  final String source;
  final String message;

  @override
  String toString() => '$source: $message';
}

/// One Markdown fragment with front matter, rendered into a target file.
class Fragment {
  const Fragment({
    required this.source,
    required this.target,
    required this.order,
    required this.rules,
    required this.when,
    required this.body,
  });

  final String source;
  final String target;
  final int order;
  final List<String> rules;
  final Map<String, Object?> when;
  final String body;
}

/// One option of a stack family, e.g. `state/bloc`.
class Pack {
  const Pack({
    required this.family,
    required this.option,
    required this.params,
    required this.fragments,
    this.skills = const {},
    this.fromProject = false,
  });

  final String family;
  final String option;
  final Map<String, Object?> params;
  final List<Fragment> fragments;

  /// Skills this pack adds, from `skills/<name>.yaml`.
  final Map<String, DocMeta> skills;

  /// True for a pack from the project's `.ai/packs/`; it replaces a built-in pack of the same family and option.
  final bool fromProject;
}

/// Description of a skill or workflow, from its `meta.yaml`.
class DocMeta {
  const DocMeta({required this.description, this.whenToUse, this.argumentHint});

  final String description;
  final String? whenToUse;
  final String? argumentHint;
}

/// `core/roles/<name>/role.yaml`: what the role may touch and how it should run.
class RoleMeta {
  const RoleMeta({
    required this.name,
    required this.description,
    required this.capability,
    this.model,
    this.effort,
    this.when = const {},
  });

  static const capabilities = {'read', 'write-plans', 'write-code'};

  final String name;
  final String description;
  final String capability;
  final String? model;
  final String? effort;

  /// Same matching as fragment `when`; the role is generated only when it matches.
  final Map<String, Object?> when;
}

/// Neutral sources: `core/` fragments, `core/roles/*/role.yaml` and `packs/<family>/<option>/`.
class Catalog {
  const Catalog({
    required this.core,
    required this.packs,
    this.roles = const {},
    this.skills = const {},
    this.workflows = const {},
  });

  final List<Fragment> core;
  final Map<String, Map<String, Pack>> packs;
  final Map<String, RoleMeta> roles;

  /// Core skill and workflow descriptions; pack skills live in [Pack.skills].
  final Map<String, DocMeta> skills;
  final Map<String, DocMeta> workflows;

  static Catalog load(String root) {
    final coreDir = Directory(p.join(root, 'core'));
    final core = coreDir.existsSync() ? _fragmentsIn(root, coreDir) : <Fragment>[];

    final roles = <String, RoleMeta>{};
    final rolesDir = Directory(p.join(root, 'core', 'roles'));
    if (rolesDir.existsSync()) {
      for (final dir in _sortedDirs(rolesDir)) {
        final manifest = File(p.join(dir.path, 'role.yaml'));
        if (manifest.existsSync()) roles[p.basename(dir.path)] = _role(root, manifest);
      }
    }

    final skills = _metasIn(root, Directory(p.join(root, 'core', 'skills')));
    final workflows = _metasIn(root, Directory(p.join(root, 'core', 'workflows')));

    final packs = _packsIn(root, 'packs', fromProject: false);
    return Catalog(core: core, packs: packs, roles: roles, skills: skills, workflows: workflows);
  }

  /// This catalog plus the packs in `<projectDir>/.ai/packs/`, which replace built-in packs of the same name.
  /// Source paths of project packs are relative to [projectDir].
  Catalog withProjectPacks(String projectDir) {
    final packs = {for (final MapEntry(:key, :value) in this.packs.entries) key: {...value}};
    _packsIn(projectDir, p.join('.ai', 'packs'), fromProject: true).forEach((family, options) {
      (packs[family] ??= {}).addAll(options);
    });
    return Catalog(core: core, packs: packs, roles: roles, skills: skills, workflows: workflows);
  }

  static Map<String, Map<String, Pack>> _packsIn(String root, String relativeDir, {required bool fromProject}) {
    final packs = <String, Map<String, Pack>>{};
    final packsDir = Directory(p.join(root, relativeDir));
    if (!packsDir.existsSync()) return packs;
    for (final familyDir in _sortedDirs(packsDir)) {
      final family = p.basename(familyDir.path);
      for (final optionDir in _sortedDirs(familyDir)) {
        final option = p.basename(optionDir.path);
        (packs[family] ??= {})[option] = _pack(root, family, option, optionDir, fromProject: fromProject);
      }
    }
    return packs;
  }

  static Map<String, DocMeta> _metasIn(String root, Directory dir) {
    if (!dir.existsSync()) return {};
    return {
      for (final sub in _sortedDirs(dir))
        if (File(p.join(sub.path, 'meta.yaml')).existsSync())
          p.basename(sub.path): _docMeta(root, File(p.join(sub.path, 'meta.yaml'))),
    };
  }

  static DocMeta _docMeta(String root, File file) {
    final source = p.relative(file.path, from: root);
    final yaml = plainYaml(loadYaml(file.readAsStringSync()));
    if (yaml is! Map<String, Object?> || yaml['description'] is! String) {
      throw CatalogException(source, 'needs a description');
    }
    return DocMeta(
      description: yaml['description'] as String,
      whenToUse: yaml['when_to_use']?.toString(),
      argumentHint: yaml['argument_hint']?.toString(),
    );
  }

  static RoleMeta _role(String root, File manifest) {
    final source = p.relative(manifest.path, from: root);
    final yaml = plainYaml(loadYaml(manifest.readAsStringSync()));
    if (yaml is! Map<String, Object?>) throw CatalogException(source, 'expected a YAML map');
    final capability = yaml['capability'];
    if (capability is! String || !RoleMeta.capabilities.contains(capability)) {
      throw CatalogException(source, 'capability must be one of ${RoleMeta.capabilities.join(', ')}');
    }
    return RoleMeta(
      name: p.basename(manifest.parent.path),
      description: yaml['description']?.toString() ?? '',
      capability: capability,
      model: yaml['model']?.toString(),
      effort: yaml['effort']?.toString(),
      when: (yaml['when'] as Map<String, Object?>?) ?? const {},
    );
  }

  Map<String, Set<String>> knownOptions() => {
        for (final MapEntry(key: family, value: options) in packs.entries) family: options.keys.toSet(),
      };

  static Pack _pack(String root, String family, String option, Directory dir, {required bool fromProject}) {
    final manifest = File(p.join(dir.path, 'pack.yaml'));
    final source = p.relative(manifest.path, from: root);
    if (option == Profile.otherOption) throw CatalogException(source, 'option name "$option" is reserved');
    if (!manifest.existsSync()) throw CatalogException(source, 'missing pack.yaml');

    final yaml = plainYaml(loadYaml(manifest.readAsStringSync()));
    if (yaml is! Map<String, Object?>) throw CatalogException(source, 'expected a YAML map');
    final params = yaml['params'] ?? <String, Object?>{};
    if (params is! Map<String, Object?>) throw CatalogException(source, 'params must be a map');

    final skillsDir = Directory(p.join(dir.path, 'skills'));
    final skills = <String, DocMeta>{
      if (skillsDir.existsSync())
        for (final file in skillsDir.listSync().whereType<File>().where((f) => f.path.endsWith('.yaml')))
          p.basenameWithoutExtension(file.path): _docMeta(root, file),
    };
    return Pack(
      family: family,
      option: option,
      params: params,
      fragments: _fragmentsIn(root, dir),
      skills: skills,
      fromProject: fromProject,
    );
  }

  static List<Directory> _sortedDirs(Directory parent) =>
      parent.listSync().whereType<Directory>().toList()..sort((a, b) => a.path.compareTo(b.path));

  static List<Fragment> _fragmentsIn(String root, Directory dir) {
    final files = dir
        .listSync(recursive: true)
        .whereType<File>()
        .where((file) => file.path.endsWith('.md'))
        .toList()
      ..sort((a, b) => a.path.compareTo(b.path));
    return [for (final file in files) _fragment(p.relative(file.path, from: root), file.readAsStringSync())];
  }

  static Fragment _fragment(String source, String text) {
    final normalized = text.replaceAll('\r\n', '\n');
    if (!normalized.startsWith('---\n')) throw CatalogException(source, 'missing front matter');
    final end = normalized.indexOf('\n---\n', 4);
    if (end < 0) throw CatalogException(source, 'unterminated front matter');

    final meta = plainYaml(loadYaml(normalized.substring(4, end)));
    if (meta is! Map<String, Object?>) throw CatalogException(source, 'front matter must be a map');

    final target = meta['target'];
    if (target is! String || target.isEmpty) throw CatalogException(source, 'front matter needs a target');
    final order = meta['order'] ?? 100;
    if (order is! int) throw CatalogException(source, 'order must be an integer');
    final rules = meta['rules'] ?? <Object?>[];
    if (rules is! List) throw CatalogException(source, 'rules must be a list');
    final when = meta['when'] ?? <String, Object?>{};
    if (when is! Map<String, Object?>) throw CatalogException(source, 'when must be a map');

    return Fragment(
      source: source,
      target: target,
      order: order,
      rules: [for (final rule in rules) rule.toString()],
      when: when,
      body: normalized.substring(end + 5),
    );
  }
}
