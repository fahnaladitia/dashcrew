import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

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
  const Pack({required this.family, required this.option, required this.params, required this.fragments});

  final String family;
  final String option;
  final Map<String, Object?> params;
  final List<Fragment> fragments;
}

/// `core/roles/<name>/role.yaml`: what the role may touch and how it should run.
class RoleMeta {
  const RoleMeta({required this.name, required this.description, required this.capability, this.model, this.effort});

  static const capabilities = {'read', 'write-plans', 'write-code'};

  final String name;
  final String description;
  final String capability;
  final String? model;
  final String? effort;
}

/// Neutral sources: `core/` fragments, `core/roles/*/role.yaml` and `packs/<family>/<option>/`.
class Catalog {
  const Catalog({required this.core, required this.packs, this.roles = const {}});

  final List<Fragment> core;
  final Map<String, Map<String, Pack>> packs;
  final Map<String, RoleMeta> roles;

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

    final packs = <String, Map<String, Pack>>{};
    final packsDir = Directory(p.join(root, 'packs'));
    if (packsDir.existsSync()) {
      for (final familyDir in _sortedDirs(packsDir)) {
        final family = p.basename(familyDir.path);
        for (final optionDir in _sortedDirs(familyDir)) {
          final option = p.basename(optionDir.path);
          (packs[family] ??= {})[option] = _pack(root, family, option, optionDir);
        }
      }
    }
    return Catalog(core: core, packs: packs, roles: roles);
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
    );
  }

  Map<String, Set<String>> knownOptions() => {
        for (final MapEntry(key: family, value: options) in packs.entries) family: options.keys.toSet(),
      };

  static Pack _pack(String root, String family, String option, Directory dir) {
    final manifest = File(p.join(dir.path, 'pack.yaml'));
    final source = p.relative(manifest.path, from: root);
    if (!manifest.existsSync()) throw CatalogException(source, 'missing pack.yaml');

    final yaml = plainYaml(loadYaml(manifest.readAsStringSync()));
    if (yaml is! Map<String, Object?>) throw CatalogException(source, 'expected a YAML map');
    final params = yaml['params'] ?? <String, Object?>{};
    if (params is! Map<String, Object?>) throw CatalogException(source, 'params must be a map');

    return Pack(family: family, option: option, params: params, fragments: _fragmentsIn(root, dir));
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
