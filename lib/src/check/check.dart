import 'dart:convert';
import 'dart:io';

import '../catalog/catalog.dart';
import '../doctor/doctor.dart';
import '../sync/sync.dart';

/// Read-only merge gate for a pull request: the toolkit is current and the PR body has every heading of the
/// generated template. Runs locally or as a required GitHub check.
class PrCheck {
  PrCheck({
    required this.catalog,
    required this.projectDir,
    this.packageRoot,
    Map<String, String>? environment,
  }) : environment = environment ?? Platform.environment;

  final Catalog catalog;
  final String projectDir;
  final String? packageRoot;

  /// `GITHUB_EVENT_PATH` supplies the PR body in CI.
  final Map<String, String> environment;

  /// [prBody] wins; otherwise the body of the pull request event in CI; without either the PR checks are skipped.
  Future<List<DoctorCheck>> run({String? prBody}) async {
    final toolkit = await Doctor(catalog: catalog, projectDir: projectDir, packageRoot: packageRoot, machineChecks: false).run();
    final body = prBody ?? _eventBody();
    return [...toolkit, _sections(body)];
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
}
