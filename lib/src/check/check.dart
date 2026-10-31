import 'dart:convert';
import 'dart:io';

import '../catalog/catalog.dart';
import '../doctor/doctor.dart';
import '../gate/gate.dart';
import '../profile/profile.dart';
import '../sync/sync.dart';

/// Comments of one GitHub issue, oldest first, as `{body, author_association, created_at, login}`.
typedef CommentsReader = Future<List<Map<String, Object?>>> Function(int issue);

/// Read-only merge gate for a pull request: the toolkit is current, the PR body has every heading of the
/// generated template, and the plan issue it names is approved. Runs locally or as a required GitHub check.
class PrCheck {
  PrCheck({
    required this.catalog,
    required this.projectDir,
    this.packageRoot,
    CommentsReader? comments,
    Map<String, String>? environment,
  })  : environment = environment ?? Platform.environment,
        comments = comments ?? _defaultReader(projectDir, environment ?? Platform.environment);

  final Catalog catalog;
  final String projectDir;
  final String? packageRoot;
  final CommentsReader comments;

  /// `GITHUB_EVENT_PATH` supplies the PR body in CI; `GITHUB_TOKEN` and `GITHUB_REPOSITORY` the API access.
  final Map<String, String> environment;

  static final _planLine = RegExp(r'^Plan:\s*#(\d+)\s*$', multiLine: true);

  /// [prBody] wins; otherwise the body of the pull request event in CI; without either the PR checks are skipped.
  Future<List<DoctorCheck>> run({String? prBody}) async {
    final toolkit = await Doctor(catalog: catalog, projectDir: projectDir, packageRoot: packageRoot, machineChecks: false).run();
    final body = prBody ?? _eventBody();
    return [...toolkit, _sections(body), await _plan(body)];
  }

  String? _eventBody() {
    final path = environment['GITHUB_EVENT_PATH'];
    if (path == null || !File(path).existsSync()) return null;
    final event = jsonDecode(File(path).readAsStringSync());
    final pullRequest = event is Map ? event['pull_request'] : null;
    return pullRequest is Map ? (pullRequest['body'] as String? ?? '') : null;
  }

  DoctorCheck _sections(String? body) {
    const id = 'pr-sections';
    final template = File('$projectDir/${Sync.prTemplatePath}');
    if (!template.existsSync()) return const DoctorCheck(id, CheckStatus.fail, '${Sync.prTemplatePath} missing; run dashcrew sync');
    if (body == null) return const DoctorCheck(id, CheckStatus.skip, 'no PR body; pass --pr-body <file> or run on a pull_request event');
    final headings = [for (final line in template.readAsLinesSync()) if (line.startsWith('## ')) line.trim()];
    final present = {for (final line in body.split('\n')) line.trim()};
    final missing = [for (final heading in headings) if (!present.contains(heading)) heading];
    if (missing.isNotEmpty) {
      return DoctorCheck(id, CheckStatus.fail, 'missing: ${missing.join(', ')} (write "None" under a heading that does not apply)');
    }
    return DoctorCheck(id, CheckStatus.ok, '${headings.length} required sections present');
  }

  Future<DoctorCheck> _plan(String? body) async {
    const id = 'plan-approval';
    if (body == null) return const DoctorCheck(id, CheckStatus.skip, 'no PR body');
    final match = _planLine.firstMatch(body);
    if (match == null) {
      if (_planGate()) return const DoctorCheck(id, CheckStatus.fail, 'plan_gate is on, so the PR body needs a "Plan: #<issue>" line');
      return const DoctorCheck(id, CheckStatus.skip, 'no plan issue named');
    }
    final issue = int.parse(match.group(1)!);
    final List<Map<String, Object?>> thread;
    try {
      thread = await comments(issue);
    } catch (error) {
      return DoctorCheck(id, CheckStatus.fail, 'could not read issue #$issue: $error');
    }
    if (Gates.approvedAt(thread) == null) {
      return DoctorCheck(id, CheckStatus.fail,
          'plan #$issue is not approved: no "/approve plan" from a collaborator after the latest plan');
    }
    return DoctorCheck(id, CheckStatus.ok, 'plan #$issue approved');
  }

  bool _planGate() {
    try {
      final profile = Profile.parse(File('$projectDir/.ai/project.yaml').readAsStringSync(),
          knownOptions: catalog.withProjectPacks(projectDir).knownOptions());
      return profile.policy['plan_gate'] == true;
    } catch (_) {
      return false;
    }
  }

  /// GitHub's REST API with `GITHUB_TOKEN` in CI; `gh` otherwise.
  static CommentsReader _defaultReader(String projectDir, Map<String, String> environment) => (issue) async {
        final token = environment['GITHUB_TOKEN'];
        final repository = environment['GITHUB_REPOSITORY'];
        if (token != null && token.isNotEmpty && repository != null) {
          return _restComments(repository, issue, token, environment['GITHUB_API_URL'] ?? 'https://api.github.com');
        }
        final result = await Process.run(
          'gh',
          ['api', 'repos/{owner}/{repo}/issues/$issue/comments', '--paginate', '--jq', '.[] | {body, author_association, created_at, login: .user.login}'],
          workingDirectory: projectDir,
        );
        if (result.exitCode != 0) throw StateError('gh: ${(result.stderr as String).trim()}');
        return Gates.parseCommentLines(result.stdout as String);
      };

  static Future<List<Map<String, Object?>>> _restComments(String repository, int issue, String token, String api) async {
    final client = HttpClient();
    final comments = <Map<String, Object?>>[];
    try {
      for (var page = 1;; page++) {
        final request = await client.getUrl(Uri.parse('$api/repos/$repository/issues/$issue/comments?per_page=100&page=$page'));
        request.headers
          ..set('Authorization', 'Bearer $token')
          ..set('Accept', 'application/vnd.github+json')
          ..set('User-Agent', 'dashcrew');
        final response = await request.close();
        final text = await response.transform(utf8.decoder).join();
        if (response.statusCode != 200) throw StateError('GitHub API ${response.statusCode}: $text');
        final batch = (jsonDecode(text) as List).cast<Map<String, Object?>>();
        comments.addAll([
          for (final c in batch)
            {'body': c['body'], 'author_association': c['author_association'], 'created_at': c['created_at'], 'login': (c['user'] as Map?)?['login']},
        ]);
        if (batch.length < 100) return comments;
      }
    } finally {
      client.close();
    }
  }
}
