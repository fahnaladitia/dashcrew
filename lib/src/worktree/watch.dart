import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import 'worktree.dart';

/// PR state of a branch as one line (`#12 OPEN`, `#12 OPEN CONFLICTING`, `#12 MERGED`), or null when there is none.
typedef PrLookup = String? Function(String branch);

/// Emits one line per new event in the watched worktrees; state in [stateDir] keeps a rerun from repeating events.
/// Events: `commit`, `pushed`, `pr`, `base-moved`, `merge-tree`, `overlap`, `error`, and a final `done`.
class Watch {
  Watch({required this.worktrees, required this.stateDir, PrLookup? prLookup}) : prLookup = prLookup ?? ghPrLookup(worktrees.mainDir);

  final Worktrees worktrees;
  final String stateDir;
  final PrLookup prLookup;

  static const doneEvent = 'done';

  /// One pass over the worktrees named in [names] (folder names), or every worktree with a recorded base.
  /// Throws [WorktreeException] for a name that is neither a worktree nor watched before.
  List<String> pass(List<String> names) {
    final state = _readState();
    final seen = (state['watched'] as List? ?? const []).cast<String>().toSet();
    final all = worktrees.list().where((info) => info.base != null).toList();
    for (final name in names) {
      if (!all.any((info) => info.name == name) && !seen.contains(name)) throw WorktreeException('No worktree named $name.');
    }
    final current = names.isEmpty ? all : all.where((info) => names.contains(info.name)).toList();
    final watched = {...seen, ...current.map((info) => info.name)};

    final events = <String>[];
    final branches = (state['branches'] as Map? ?? {}).cast<String, Object?>();
    final bases = (state['bases'] as Map? ?? {}).cast<String, Object?>();

    for (final base in current.map((info) => info.base!).toSet().toList()..sort()) {
      final fetch = _git(['fetch', '--quiet', 'origin', base]);
      if (fetch.exitCode != 0) {
        events.add('error origin/$base: fetch failed: ${(fetch.stderr as String).trim().split('\n').first}');
        continue;
      }
      final sha = _revParse('refs/remotes/origin/$base');
      final previous = bases[base] as String?;
      bases[base] = sha;
      if (previous == null || previous == sha) continue;
      events.add('base-moved origin/$base: ${_short(previous)}..${_short(sha)}');
      for (final info in current.where((info) => info.base == base)) {
        events.add('merge-tree ${info.branch} vs origin/$base: ${_mergeTree(base, info.branch)}');
      }
    }

    for (final info in current) {
      final previous = (branches[info.branch] as Map? ?? {}).cast<String, Object?>();
      final next = <String, Object?>{...previous};
      final ahead = int.tryParse((_git(['rev-list', '--count', 'origin/${info.base}..${info.branch}']).stdout as String).trim()) ?? 0;
      if (ahead > 0 && ((previous['ahead'] as int?) ?? 0) == 0) {
        events.add('commit ${info.branch}: first commit, $ahead ahead of origin/${info.base}');
      }
      next['ahead'] = ahead;

      final remote = (_git(['ls-remote', 'origin', 'refs/heads/${info.branch}']).stdout as String).trim();
      final pushed = remote.isEmpty ? null : remote.split(RegExp(r'\s+')).first;
      if (pushed != null && pushed != previous['pushed']) events.add('pushed ${info.branch}: ${_short(pushed)}');
      next['pushed'] = pushed;

      final pr = prLookup(info.branch);
      if (pr != null && pr != previous['pr']) events.add('pr ${info.branch}: $pr');
      next['pr'] = pr;
      branches[info.branch] = next;
    }

    final overlaps = (state['overlaps'] as Map? ?? {}).cast<String, Object?>();
    final changed = {for (final info in current) info.branch: _changedFiles(info)};
    final sorted = changed.keys.toList()..sort();
    for (var i = 0; i < sorted.length; i++) {
      for (var j = i + 1; j < sorted.length; j++) {
        final key = '${sorted[i]} + ${sorted[j]}';
        final files = changed[sorted[i]]!.intersection(changed[sorted[j]]!).toList()..sort();
        final before = (overlaps[key] as List? ?? const []).cast<String>().toSet();
        if (files.any((file) => !before.contains(file))) events.add('overlap $key: ${files.join(', ')}');
        overlaps[key] = files;
      }
    }

    final branchByName = {for (final info in current) info.name: info.branch};
    final merged = [
      for (final name in watched)
        if (!branchByName.containsKey(name) || ((branches[branchByName[name]] as Map?)?['pr'] as String? ?? '').contains('MERGED')) name,
    ];
    if (watched.isNotEmpty && merged.length == watched.length) {
      events.add('$doneEvent: ${watched.length} worktrees merged or removed');
    }

    _writeState({'watched': watched.toList()..sort(), 'bases': bases, 'branches': branches, 'overlaps': overlaps});
    return events;
  }

  /// Passes every [interval] until `done`; [once] stops after the first pass.
  Future<void> run(List<String> names, {required bool once, required Duration interval, required void Function(String) out}) async {
    while (true) {
      final events = pass(names);
      events.forEach(out);
      if (once || events.any((event) => event.startsWith(doneEvent))) return;
      await Future<void>.delayed(interval);
    }
  }

  /// `gh pr list` for the branch; null when gh is missing, fails, or finds no PR.
  static PrLookup ghPrLookup(String mainDir) => (branch) {
        final ProcessResult result;
        try {
          result = Process.runSync(
            'gh',
            ['pr', 'list', '--head', branch, '--state', 'all', '--limit', '1', '--json', 'number,state,mergeable'],
            workingDirectory: mainDir,
          );
        } on ProcessException {
          return null;
        }
        if (result.exitCode != 0) return null;
        final list = jsonDecode(result.stdout as String) as List;
        if (list.isEmpty) return null;
        final pr = list.first as Map;
        return '#${pr['number']} ${pr['state']}${pr['mergeable'] == 'CONFLICTING' ? ' CONFLICTING' : ''}';
      };

  Set<String> _changedFiles(WorktreeInfo info) {
    final committed = (_git(['diff', '--name-only', 'origin/${info.base}...${info.branch}']).stdout as String).trim();
    final working = (Process.runSync('git', ['status', '--porcelain'], workingDirectory: info.path).stdout as String)
        .split('\n')
        .where((line) => line.length > 3);
    return {
      if (committed.isNotEmpty) ...committed.split('\n'),
      ...working.map((line) => line.substring(3)),
    };
  }

  String _mergeTree(String base, String branch) {
    final result = _git(['merge-tree', '--write-tree', '--name-only', '--no-messages', 'origin/$base', branch]);
    if (result.exitCode == 0) return 'CLEAN';
    final lines = (result.stdout as String).trim().split('\n');
    if (result.exitCode == 1 && lines.length > 1) return 'CONFLICT ${lines.skip(1).join(', ')}';
    return 'ERROR ${(result.stderr as String).trim().split('\n').first}';
  }

  String _revParse(String ref) => (_git(['rev-parse', ref]).stdout as String).trim();

  static String _short(String sha) => sha.length > 7 ? sha.substring(0, 7) : sha;

  ProcessResult _git(List<String> args) => Process.runSync('git', args, workingDirectory: worktrees.mainDir);

  File get _stateFile => File(p.join(stateDir, 'state.json'));

  Map<String, Object?> _readState() =>
      _stateFile.existsSync() ? jsonDecode(_stateFile.readAsStringSync()) as Map<String, Object?> : <String, Object?>{};

  void _writeState(Map<String, Object?> state) {
    _stateFile
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(const JsonEncoder.withIndent('  ').convert(state));
  }
}
