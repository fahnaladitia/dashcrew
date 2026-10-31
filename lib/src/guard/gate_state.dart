import 'dart:convert';
import 'dart:io';

/// The plan gate of the checked-out branch, as `dashcrew gate` recorded it.
typedef PlanGateState = ({String branch, String plan, int? issue});

/// Reads the branch from `.git/HEAD` (or a worktree's git dir) and its entry in `<common dir>/dashcrew/gates.json`,
/// without running git, so the guard stays fast. Null on a detached HEAD, outside git, or with nothing recorded.
PlanGateState? readPlanGateState(String projectRoot) {
  try {
    final dotGit = '$projectRoot/.git';
    String gitDir;
    if (FileSystemEntity.isDirectorySync(dotGit)) {
      gitDir = dotGit;
    } else if (FileSystemEntity.isFileSync(dotGit)) {
      final pointer = File(dotGit).readAsStringSync().trim();
      if (!pointer.startsWith('gitdir: ')) return null;
      gitDir = _resolve(projectRoot, pointer.substring('gitdir: '.length).trim());
    } else {
      return null;
    }

    final head = File('$gitDir/HEAD').readAsStringSync().trim();
    if (!head.startsWith('ref: refs/heads/')) return null;
    final branch = head.substring('ref: refs/heads/'.length);

    final commondirFile = File('$gitDir/commondir');
    final commonDir = commondirFile.existsSync() ? _resolve(gitDir, commondirFile.readAsStringSync().trim()) : gitDir;
    final states = File('$commonDir/dashcrew/gates.json');
    if (!states.existsSync()) return null;
    final entry = (jsonDecode(states.readAsStringSync()) as Map)[branch];
    if (entry is! Map) return null;
    return (branch: branch, plan: entry['plan'] as String? ?? 'none', issue: entry['issue'] as int?);
  } catch (_) {
    return null;
  }
}

String _resolve(String base, String path) => path.startsWith('/') ? path : '$base/$path';
