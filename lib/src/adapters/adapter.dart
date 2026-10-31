import 'dart:convert';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/project_files.dart';
import '../sync/sync.dart';
import 'antigravity.dart';
import 'claude_code.dart';
import 'codex.dart';

/// What `sync` writes for one agent named in `project.yaml` `agents`; AGENTS.md and the guard policy are shared.
abstract class Adapter {
  const Adapter();

  /// The `agents` value this adapter serves.
  String get agent;

  /// Generated files (relative path -> content), tracked in the lock and removed once no longer generated.
  /// [lockedFiles] tells a file dashcrew wrote earlier apart from one the user owns.
  Map<String, String> files(
    Profile profile,
    List<RenderedTarget> targets,
    Map<String, Object?> context, {
    required Map<String, String> lockedFiles,
    List<String>? warnings,
  });

  /// Adds dashcrew entries to files the user edits too, keeping everything else.
  /// [ownedMcp] are the MCP servers this adapter wrote before; the result says which it owns now.
  ({List<String> written, Set<String> ownedMcp}) merge(Set<String> ownedMcp, List<String> warnings);
}

/// A skill or workflow ready to write: its description and rendered body.
typedef SkillDoc = ({String name, DocMeta meta, String body});

/// Skills and workflows among [targets], for every adapter alike.
/// A skill no selected pack describes is skipped with a warning (a project pack replaced its owner);
/// a workflow without a description throws [StateError].
List<SkillDoc> skillDocs(Catalog catalog, Profile profile, List<RenderedTarget> targets, List<String>? warnings) {
  final packSkills = {for (final pack in Renderer(catalog).selectedPacks(profile)) ...pack.skills};
  final docs = <SkillDoc>[];
  for (final target in targets) {
    final slash = target.target.indexOf('/');
    if (slash < 0) continue;
    final kind = target.target.substring(0, slash);
    final name = target.target.substring(slash + 1);
    if (kind == 'skill') {
      final meta = catalog.skills[name] ?? packSkills[name];
      if (meta == null) {
        warnings?.add('skill $name skipped: no selected pack defines skills/$name.yaml');
        continue;
      }
      docs.add((name: name, meta: meta, body: target.content));
    } else if (kind == 'workflow') {
      final meta = catalog.workflows[name];
      if (meta == null) throw StateError('workflow/$name has no meta.yaml');
      docs.add((name: name, meta: meta, body: target.content));
    }
  }
  return docs;
}

/// `.agents/skills/<name>/SKILL.md`, the format Codex and Antigravity both read: front matter `name` and
/// `description` only, so the trigger joins the description and the argument hint opens the body.
String agentsSkillFile(String name, DocMeta meta, String body) {
  final description = meta.whenToUse == null ? meta.description : '${meta.description} Use when: ${meta.whenToUse}';
  final arguments = meta.argumentHint == null ? '' : 'Arguments: `${meta.argumentHint}`\n\n';
  return '---\nname: $name\ndescription: ${jsonEncode(description)}\n---\n\n$arguments$body';
}

/// Adds [servers] to the `mcpServers` map of the JSON file at [path], keeping every other entry.
/// A server is replaced only when dashcrew [owned] it before; one the user defined differently stays, with a warning.
({bool changed, Set<String> owned}) mergeJsonMcpServers(
  ProjectFiles project,
  String path,
  Map<String, Map<String, Object?>> servers,
  Set<String> owned,
  List<String> warnings,
) {
  final file = project.file(path);
  final config = file.existsSync() ? jsonDecode(file.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};
  final current = (config['mcpServers'] as Map?)?.cast<String, Object?>() ?? <String, Object?>{};
  final nowOwned = {...owned};

  servers.forEach((name, server) {
    final existing = current[name];
    if (existing == null || owned.contains(name)) {
      current[name] = server;
      nowOwned.add(name);
    } else if (jsonEncode(existing) != jsonEncode(server)) {
      warnings.add('$path already defines "$name" differently; left as is.');
    }
  });
  config['mcpServers'] = current;
  final changed = project.writeIfChanged(path, '${const JsonEncoder.withIndent('  ').convert(config)}\n');
  return (changed: changed, owned: nowOwned);
}

/// Adapter for [agent]; throws [SyncException] for an agent that has none yet.
Adapter adapterFor(String agent, Catalog catalog, String projectDir) => switch (agent) {
      'claude-code' => ClaudeCodeAdapter(catalog, projectDir: projectDir),
      'codex' => CodexAdapter(catalog, projectDir: projectDir),
      'antigravity' => AntigravityAdapter(catalog, projectDir: projectDir),
      _ => throw SyncException('Agent adapter not available yet: $agent. Supported: ${supportedAdapters.join(', ')}.'),
    };

/// Agents [adapterFor] accepts.
const supportedAdapters = {'claude-code', 'codex', 'antigravity'};
