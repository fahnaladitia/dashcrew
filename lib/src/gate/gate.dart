import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

/// A gate command refused to run; [message] says why and what to do.
class GateException implements Exception {
  GateException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// Outcome of one `gh` call.
typedef GhResult = ({int exitCode, String stdout, String stderr});

/// Runs `gh` with [args] in the project.
typedef GhRunner = GhResult Function(List<String> args);

/// Recorded state of one branch's plan gate: `none`, `pending` or `approved`.
typedef GateState = ({int? issue, String plan, String? at});

/// Plan approval on the branch's GitHub issue: the agent posts the plan and stops; a repository collaborator
/// answers `/approve plan`. State is kept per branch in `<git common dir>/dashcrew/gates.json`, which the guard reads.
class Gates {
  Gates({required this.projectDir, GhRunner? gh}) : gh = gh ?? _runGh(projectDir);

  final String projectDir;
  final GhRunner gh;

  /// First line of every plan comment; an approval counts only after the latest one.
  static const planMarker = '<!-- dashcrew:plan -->';
  static const _approval = '/approve plan';
  static const _approvers = {'OWNER', 'MEMBER', 'COLLABORATOR'};

  /// Posts [planFile] to the branch's issue (an explicit [issue], else the linked one, else a new one) and records
  /// the branch as pending.
  ({int issue, String url}) submitPlan(String planFile, {int? issue}) {
    final file = File(planFile);
    if (!file.existsSync()) throw GateException('Plan file $planFile not found.');
    final branch = _branch();
    var number = issue ?? _linkedIssue(branch);
    if (number == null) {
      final created = _gh(['issue', 'create', '--title', 'Plan: $branch', '--body', 'Plan gate for branch `$branch`. '
          'Approve with a comment whose first line is `$_approval`.']);
      number = _issueNumber(created);
    }
    final body = File(p.join(Directory.systemTemp.createTempSync('dashcrew_plan_').path, 'plan.md'))
      ..writeAsStringSync('$planMarker\n${file.readAsStringSync()}');
    final String posted;
    try {
      posted = _gh(['issue', 'comment', '$number', '--body-file', body.path]);
    } finally {
      body.parent.deleteSync(recursive: true);
    }
    _git(['config', 'branch.$branch.dashcrew-issue', '$number']);
    _writeState(branch, (issue: number, plan: 'pending', at: DateTime.now().toUtc().toIso8601String()));
    return (issue: number, url: posted.trim());
  }

  /// Reads the linked issue's comments and records whether the latest plan was approved; `none` without an issue.
  GateState status() {
    final branch = _branch();
    final issue = _linkedIssue(branch);
    if (issue == null) return (issue: null, plan: 'none', at: null);

    final output = _gh([
      'api', 'repos/{owner}/{repo}/issues/$issue/comments', '--paginate',
      '--jq', '.[] | {body, author_association, created_at, login: .user.login}',
    ]);
    final comments = [
      for (final line in output.split('\n'))
        if (line.trim().isNotEmpty) jsonDecode(line) as Map<String, Object?>,
    ];
    final lastPlan = comments.lastIndexWhere((c) => (c['body'] as String? ?? '').startsWith(planMarker));
    String? approvedAt;
    if (lastPlan >= 0) {
      for (final c in comments.skip(lastPlan + 1)) {
        final firstLine = (c['body'] as String? ?? '').split('\n').first.trim();
        if (firstLine == _approval && _approvers.contains(c['author_association'])) approvedAt = c['created_at'] as String?;
      }
    }
    final state = (issue: issue, plan: approvedAt == null ? 'pending' : 'approved', at: approvedAt);
    _writeState(branch, state);
    return state;
  }

  /// Last recorded state of [branch]; null when none was recorded.
  GateState? stateOf(String branch) {
    final entry = _readStates()[branch];
    if (entry is! Map) return null;
    return (issue: entry['issue'] as int?, plan: entry['plan'] as String? ?? 'none', at: entry['at'] as String?);
  }

  String _branch() {
    final branch = _git(['branch', '--show-current']).trim();
    if (branch.isEmpty) throw GateException('No branch checked out; gates are kept per branch.');
    return branch;
  }

  int? _linkedIssue(String branch) {
    final result = Process.runSync('git', ['config', '--get', 'branch.$branch.dashcrew-issue'], workingDirectory: projectDir);
    return result.exitCode == 0 ? int.tryParse((result.stdout as String).trim()) : null;
  }

  static int _issueNumber(String url) {
    final match = RegExp(r'/issues/(\d+)').firstMatch(url);
    if (match == null) throw GateException('Could not read the issue number from gh: ${url.trim()}');
    return int.parse(match.group(1)!);
  }

  File get _stateFile =>
      File(p.join(_git(['rev-parse', '--path-format=absolute', '--git-common-dir']).trim(), 'dashcrew', 'gates.json'));

  Map<String, Object?> _readStates() {
    final file = _stateFile;
    if (!file.existsSync()) return {};
    final decoded = jsonDecode(file.readAsStringSync());
    return decoded is Map<String, Object?> ? decoded : {};
  }

  void _writeState(String branch, GateState state) {
    final states = _readStates()..[branch] = {'issue': state.issue, 'plan': state.plan, 'at': state.at};
    _stateFile
      ..parent.createSync(recursive: true)
      ..writeAsStringSync('${const JsonEncoder.withIndent('  ').convert(states)}\n');
  }

  String _gh(List<String> args) {
    final result = gh(args);
    if (result.exitCode != 0) throw GateException('gh ${args.take(2).join(' ')} failed: ${result.stderr.trim()}');
    return result.stdout;
  }

  String _git(List<String> args) {
    final result = Process.runSync('git', args, workingDirectory: projectDir);
    if (result.exitCode != 0) throw GateException('git ${args.join(' ')} failed: ${(result.stderr as String).trim()}');
    return result.stdout as String;
  }

  static GhRunner _runGh(String projectDir) => (args) {
        try {
          final result = Process.runSync('gh', args, workingDirectory: projectDir);
          return (exitCode: result.exitCode, stdout: result.stdout as String, stderr: result.stderr as String);
        } on ProcessException catch (error) {
          return (exitCode: 127, stdout: '', stderr: 'gh not found: ${error.message}');
        }
      };
}
