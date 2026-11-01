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
  const DocMeta({required this.description, this.whenToUse, this.argumentHint, this.review = false});

  final String description;
  final String? whenToUse;
  final String? argumentHint;

  /// `review: true` marks a skill whose checklist the reviewer applies when the skill is active for the task.
  final bool review;
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

/// An MCP server a platform's agents use; each adapter writes it in its own format.
typedef McpServer = ({String command, List<String> args});

/// One platform: `platforms/<name>/` in the catalog, `.ai/platforms/<name>/` in a project, or `other` from the profile.
/// Its fragments and metas render only when the profile selects it.
class PlatformSpec {
  const PlatformSpec({
    required this.name,
    required this.detect,
    required this.commands,
    this.root = '',
    this.description,
    this.link,
    this.docComment,
    this.mcpServers = const {},
    this.fragments = const [],
    this.skills = const {},
    this.workflows = const {},
    this.packs = const {},
    this.fromProject = false,
  });

  static const _keys = {'description', 'detect', 'commands', 'link', 'doc_comment', 'mcp_servers'};

  final String name;

  /// Source folder of the platform (`platforms/flutter`); empty for `other`.
  final String root;
  final String? description;

  /// Files whose presence identifies the platform, relative to the project root.
  final List<String> detect;

  /// [Profile.platformCommands] plus `format`, which may be null.
  final Map<String, String?> commands;

  /// How repositories link a local package, such as `pubspec_overrides.yaml`; null when there is none.
  final String? link;

  /// The doc comment convention, such as Dartdoc; null when the platform names none.
  final String? docComment;

  /// MCP servers `sync` adds to each agent's config, in declaration order.
  final Map<String, McpServer> mcpServers;

  /// Fragments outside `packs/`; one at the same path as a core fragment replaces it.
  final List<Fragment> fragments;
  final Map<String, DocMeta> skills;
  final Map<String, DocMeta> workflows;

  /// The platform's own pack families, resolved before the shared `packs/`.
  final Map<String, Map<String, Pack>> packs;
  final bool fromProject;

  /// `platform.*` in templates.
  Map<String, Object?> toContext() => {
        'name': name,
        'description': description,
        'detect': detect,
        'commands': commands,
        'link': link,
        'doc_comment': docComment,
      };

  /// Core sources the platform replaces: `core/<path>` for each fragment at `<root>/<path>`.
  Set<String> get replacedCoreSources => {
        for (final fragment in fragments) p.join('core', p.relative(fragment.source, from: root)),
      };
}

/// Neutral sources: `core/` fragments, `core/roles/*/role.yaml`, the shared `packs/<family>/<option>/`
/// and `platforms/<name>/`.
class Catalog {
  const Catalog({
    required this.core,
    required this.packs,
    this.roles = const {},
    this.skills = const {},
    this.workflows = const {},
    this.platforms = const {},
  });

  final List<Fragment> core;

  /// Shared pack families, plus the project's `.ai/packs/` after [withProjectPacks].
  final Map<String, Map<String, Pack>> packs;
  final Map<String, RoleMeta> roles;

  /// Core skill and workflow descriptions; pack skills live in [Pack.skills], platform ones in [PlatformSpec].
  final Map<String, DocMeta> skills;
  final Map<String, DocMeta> workflows;

  /// Built-in platforms, plus the project's `.ai/platforms/` after [withProjectPacks].
  final Map<String, PlatformSpec> platforms;

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
    final platforms = _platformsIn(root, 'platforms', fromProject: false);
    return Catalog(core: core, packs: packs, roles: roles, skills: skills, workflows: workflows, platforms: platforms);
  }

  /// This catalog plus the packs in `<projectDir>/.ai/packs/`, which replace built-in and platform packs of the
  /// same name, and the platforms in `<projectDir>/.ai/platforms/`, which replace a built-in platform of the same name.
  /// Source paths of project packs and platforms are relative to [projectDir].
  Catalog withProjectPacks(String projectDir) {
    final packs = {for (final MapEntry(:key, :value) in this.packs.entries) key: {...value}};
    _packsIn(projectDir, p.join('.ai', 'packs'), fromProject: true).forEach((family, options) {
      (packs[family] ??= {}).addAll(options);
    });
    final platforms = {...this.platforms, ..._platformsIn(projectDir, p.join('.ai', 'platforms'), fromProject: true)};
    return Catalog(core: core, packs: packs, roles: roles, skills: skills, workflows: workflows, platforms: platforms);
  }

  /// Packs a profile on [platform] may select: the platform's families over the shared ones;
  /// a project pack from `.ai/packs/` wins over both.
  Map<String, Map<String, Pack>> packsFor(String platform) {
    final merged = {for (final MapEntry(:key, :value) in packs.entries) key: {...value}};
    platforms[platform]?.packs.forEach((family, options) {
      final target = merged[family] ??= {};
      options.forEach((option, pack) {
        if (target[option]?.fromProject != true) target[option] = pack;
      });
    });
    return merged;
  }

  /// This catalog with [packs] resolved by [packsFor], for code that reads `packs` directly (the guard policy).
  Catalog resolvedFor(String platform) => Catalog(
        core: core,
        packs: packsFor(platform),
        roles: roles,
        skills: skills,
        workflows: workflows,
        platforms: platforms,
      );

  /// Core fragments, minus those the platform replaces, then the platform's own; [Renderer] orders them.
  List<Fragment> fragmentsFor(String platform) {
    final selected = platforms[platform];
    if (selected == null) return core;
    final replaced = selected.replacedCoreSources;
    return [...core.where((fragment) => !replaced.contains(fragment.source)), ...selected.fragments];
  }

  /// Core skill descriptions with the platform's over them.
  Map<String, DocMeta> skillsFor(String platform) => {...skills, ...?platforms[platform]?.skills};

  /// Core workflow descriptions with the platform's over them.
  Map<String, DocMeta> workflowsFor(String platform) => {...workflows, ...?platforms[platform]?.workflows};

  /// The profile's platform; `other` comes from `platform_config`, a name the catalog lacks gives an empty platform.
  PlatformSpec platformFor(Profile profile) {
    final config = profile.platformConfig;
    if (profile.platform == Profile.otherOption && config != null) {
      return PlatformSpec(
        name: Profile.otherOption,
        detect: (config['detect'] as List).cast<String>(),
        commands: (config['commands'] as Map).cast<String, String?>(),
      );
    }
    return platforms[profile.platform] ?? PlatformSpec(name: profile.platform, detect: const [], commands: const {});
  }

  static Map<String, PlatformSpec> _platformsIn(String root, String relativeDir, {required bool fromProject}) {
    final dir = Directory(p.join(root, relativeDir));
    if (!dir.existsSync()) return {};
    return {
      for (final platformDir in _sortedDirs(dir))
        p.basename(platformDir.path): _platform(root, platformDir, fromProject: fromProject),
    };
  }

  static PlatformSpec _platform(String root, Directory dir, {required bool fromProject}) {
    final name = p.basename(dir.path);
    final manifest = File(p.join(dir.path, 'platform.yaml'));
    final source = p.relative(manifest.path, from: root);
    if (name == Profile.otherOption) throw CatalogException(source, 'platform name "$name" is reserved');
    if (!manifest.existsSync()) throw CatalogException(source, 'missing platform.yaml');

    final yaml = plainYaml(loadYaml(manifest.readAsStringSync()));
    if (yaml is! Map<String, Object?>) throw CatalogException(source, 'expected a YAML map');
    for (final key in yaml.keys) {
      if (!PlatformSpec._keys.contains(key)) throw CatalogException(source, 'unknown key $key');
    }
    final detect = yaml['detect'];
    if (detect is! List || detect.isEmpty || detect.any((file) => file is! String || file.isEmpty)) {
      throw CatalogException(source, 'detect: required, a non-empty list of files that identify the platform');
    }
    final commands = platformCommandsFrom(yaml['commands'], (key, message) => throw CatalogException(source, '$key: $message'));
    for (final key in ['description', 'link', 'doc_comment']) {
      if (yaml[key] != null && yaml[key] is! String) throw CatalogException(source, '$key: expected a string');
    }

    final mcpServers = <String, McpServer>{};
    final rawServers = yaml['mcp_servers'] ?? <String, Object?>{};
    if (rawServers is! Map<String, Object?>) throw CatalogException(source, 'mcp_servers: expected a map of name to command and args');
    rawServers.forEach((name, server) {
      final command = server is Map ? server['command'] : null;
      final args = server is Map ? server['args'] ?? <Object?>[] : null;
      if (command is! String || command.isEmpty || args is! List || args.any((arg) => arg is! String)) {
        throw CatalogException(source, 'mcp_servers.$name: expected { command: <string>, args: [<string>, ...] }');
      }
      mcpServers[name] = (command: command, args: args.cast<String>());
    });

    final rootPath = p.relative(dir.path, from: root);
    final packsDir = p.join(rootPath, 'packs');
    return PlatformSpec(
      name: name,
      root: rootPath,
      description: yaml['description'] as String?,
      detect: detect.cast<String>(),
      commands: commands,
      link: yaml['link'] as String?,
      docComment: yaml['doc_comment'] as String?,
      mcpServers: mcpServers,
      fragments: [
        for (final fragment in _fragmentsIn(root, dir))
          if (!p.isWithin(packsDir, fragment.source)) fragment,
      ],
      skills: _metasIn(root, Directory(p.join(dir.path, 'skills'))),
      workflows: _metasIn(root, Directory(p.join(dir.path, 'workflows'))),
      packs: _packsIn(root, packsDir, fromProject: fromProject),
      fromProject: fromProject,
    );
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
    final review = yaml['review'] ?? false;
    if (review is! bool) throw CatalogException(source, 'review: expected true or false');
    return DocMeta(
      description: yaml['description'] as String,
      whenToUse: yaml['when_to_use']?.toString(),
      argumentHint: yaml['argument_hint']?.toString(),
      review: review,
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

  /// Platform name -> the families and options a profile on it may select; [Profile.defaultPlatform] and
  /// [Profile.otherOption] are always present (`other` sees the shared families only).
  Map<String, Map<String, Set<String>>> knownOptions() => {
        for (final platform in {Profile.defaultPlatform, ...platforms.keys, Profile.otherOption})
          platform: {
            for (final MapEntry(key: family, value: options) in packsFor(platform).entries) family: options.keys.toSet(),
          },
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
