import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Parses the Codex TOML files of the `sample-codex` golden with Python's `tomllib`, so escaping is checked
/// by a real TOML parser; skipped when no Python 3.11+ is on PATH.
void main() {
  final expected = p.join('test', 'golden', 'sample-codex', 'expected');

  String? python() {
    for (final candidate in ['python3.14', 'python3.13', 'python3.12', 'python3.11', 'python3']) {
      try {
        if (Process.runSync(candidate, ['-c', 'import tomllib']).exitCode == 0) return candidate;
      } on ProcessException {
        continue;
      }
    }
    return null;
  }

  const script = r'''
import glob, os, sys, tomllib
root = sys.argv[1]
problems = []
for path in sorted(glob.glob(os.path.join(root, '.codex', 'agents', '*.toml'))):
    name = os.path.basename(path)[:-5]
    with open(path, 'rb') as f:
        data = tomllib.load(f)
    if data.get('name') != name or not data.get('description', '').strip():
        problems.append(f'{name}: name or description')
    claude = open(os.path.join(root, '.claude', 'agents', name + '.md')).read().split('\n---\n', 1)[1].lstrip('\n')
    if data.get('developer_instructions') != claude:
        problems.append(f'{name}: developer_instructions differ from the claude code body')
with open(os.path.join(root, '.codex', 'config.toml'), 'rb') as f:
    if 'dart-mcp-server' not in tomllib.load(f).get('mcp_servers', {}):
        problems.append('config.toml: dart-mcp-server missing')
print('\n'.join(problems))
sys.exit(1 if problems else 0)
''';

  test('codex role and config files parse as TOML and keep the role bodies', () {
    final interpreter = python();
    if (interpreter == null) {
      markTestSkipped('no Python 3.11+ with tomllib on PATH');
      return;
    }

    final result = Process.runSync(interpreter, ['-I', '-c', script, expected]);

    expect(result.exitCode, 0, reason: '${result.stdout}${result.stderr}');
  });
}
