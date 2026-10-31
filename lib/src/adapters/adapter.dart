import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../render/renderer.dart';
import '../sync/sync.dart';
import 'claude_code.dart';

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

/// Adapter for [agent]; throws [SyncException] for an agent that has none yet.
Adapter adapterFor(String agent, Catalog catalog, String projectDir) => switch (agent) {
      'claude-code' => ClaudeCodeAdapter(catalog, projectDir: projectDir),
      _ => throw SyncException('Agent adapter not available yet: $agent. Supported: ${supportedAdapters.join(', ')}.'),
    };

/// Agents [adapterFor] accepts.
const supportedAdapters = {'claude-code'};
