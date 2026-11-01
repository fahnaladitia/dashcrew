import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Runs the generated `.ai/bin/guard` shim through `sh` with a stand-in `dashcrew` on PATH. The stand-in answers
/// `--version` per STANDIN_VERSION ("dart" is the Dart usage error) after reading stdin, which the shim must not
/// hand it, and for `guard` records its arguments and stdin, prints a line on each stream and exits with
/// STANDIN_GUARD_EXIT. Each --version call adds a line to STANDIN_DIR/versions. PATH holds only `cat`, `mkdir` and,
/// unless a run has no dashcrew, the stand-in; HOME is a temp folder, where the shim caches a passed check.
void main() {
  late Directory root;
  late String shim;
  late String bin;
  late String tools;
  late String home;
  late String cachePath;

  const standIn = r'''
#!/bin/sh
if [ "$1" = --version ]; then
  cat >/dev/null
  echo called >> "$STANDIN_DIR/versions"
  case $STANDIN_VERSION in
    dart) echo 'Could not find an option named "version".' >&2; exit 64 ;;
    '') exit 0 ;;
  esac
  echo "dashcrew $STANDIN_VERSION"
  exit 0
fi
if [ "$1" = guard ]; then
  shift
  printf '[%s]' "$@" > "$STANDIN_DIR/args"
  cat > "$STANDIN_DIR/stdin"
  echo guard-out
  echo guard-err >&2
  exit "$STANDIN_GUARD_EXIT"
fi
exit 64
''';

  void writeExecutable(String path, String content) {
    File(path)
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(content);
    Process.runSync('chmod', ['+x', path]);
  }

  Future<ProcessResult> runShim(
    String input, {
    bool missing = false,
    String version = '',
    int guardExit = 0,
    List<String> args = const ['policy.json'],
    bool keepCache = false,
  }) async {
    if (!keepCache && Directory(p.join(home, '.dashcrew')).existsSync()) {
      Directory(p.join(home, '.dashcrew')).deleteSync(recursive: true);
    }
    for (final record in ['args', 'stdin', 'versions']) {
      final file = File(p.join(root.path, record));
      if (file.existsSync()) file.deleteSync();
    }
    final process = await Process.start('/bin/sh', [shim, ...args], includeParentEnvironment: false, environment: {
      'PATH': missing ? tools : '$bin:$tools',
      'HOME': home,
      'STANDIN_DIR': root.path,
      'STANDIN_VERSION': version,
      'STANDIN_GUARD_EXIT': '$guardExit',
    });
    process.stdin.write(input);
    await process.stdin.close();
    final out = await process.stdout.transform(const SystemEncoding().decoder).join();
    final err = await process.stderr.transform(const SystemEncoding().decoder).join();
    return ProcessResult(process.pid, await process.exitCode, out, err);
  }

  String? recorded(String name) {
    final file = File(p.join(root.path, name));
    return file.existsSync() ? file.readAsStringSync() : null;
  }

  const claudePush = '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git push"}}';

  setUp(() {
    root = Directory(Directory.systemTemp.createTempSync('dashcrew_shim_').resolveSymbolicLinksSync());
    shim = p.join(root.path, 'guard');
    bin = p.join(root.path, 'bin');
    tools = p.join(root.path, 'tools');
    home = p.join(root.path, 'home');
    cachePath = p.join(home, '.dashcrew', 'guard-ok');
    Directory(home).createSync();
    writeExecutable(shim, guardShim('0.1.0'));
    writeExecutable(p.join(bin, 'dashcrew'), standIn);
    Directory(tools).createSync();
    for (final tool in ['cat', 'mkdir']) {
      final found = (Process.runSync('/bin/sh', ['-c', 'command -v $tool']).stdout as String).trim();
      Link(p.join(tools, tool)).createSync(found);
    }
  });
  tearDown(() => root.deleteSync(recursive: true));

  test('the shim is the one the goldens record', () {
    expect(guardShim(guardMinimumVersion), File('test/golden/sample-codex/expected/.ai/bin/guard').readAsStringSync());
  });

  group('refuses with exit 2 and the install hint', () {
    final cases = <String, ({bool missing, String version})>{
      'missing': (missing: true, version: ''),
      'dart without --version': (missing: false, version: 'dart'),
      'dart 0.0.1': (missing: false, version: '0.0.1'),
      'pre-release of the minimum': (missing: false, version: '0.1.0-dev.1'),
      'older': (missing: false, version: '0.0.9'),
      'two parts': (missing: false, version: '1.2'),
      'four parts': (missing: false, version: '1.2.3.4'),
      'not a number': (missing: false, version: '1.x.0'),
      'no output': (missing: false, version: ''),
    };
    for (final MapEntry(key: name, value: run) in cases.entries) {
      test(name, () async {
        final result = await runShim(claudePush, missing: run.missing, version: run.version);

        expect(result.exitCode, 2);
        expect(result.stdout, isEmpty);
        expect(recorded('args'), isNull);
        expect(result.stderr, startsWith('[dashcrew guard] '));
        expect(result.stderr,
            contains('this tool call is refused. Install dashcrew 0.1.0 or newer (brew install fahnaladitia/tap/dashcrew'));
        expect('\n'.allMatches(result.stderr as String).length, 1);
        expect(File(cachePath).existsSync(), isFalse, reason: 'a refusal wrote the cache');
      });
    }
  });

  test('names the reason', () async {
    expect((await runShim(claudePush, missing: true)).stderr, contains('dashcrew is not on PATH, so'));
    expect((await runShim(claudePush, version: '0.0.1')).stderr,
        contains('the dashcrew on PATH is older than 0.1.0 or reports no version, so'));
  });

  test('refuses Antigravity with a deny decision on stdout', () async {
    final result = await runShim('{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}}}', missing: true);

    expect(result.exitCode, 0);
    expect(result.stderr, isEmpty);
    expect(result.stdout, startsWith('{"decision":"deny","reason":"[dashcrew guard] dashcrew is not on PATH'));
    expect(result.stdout, endsWith('"}\n'));
  });

  test('keeps exit 2 for a Claude payload that mentions toolCall', () async {
    final result =
        await runShim('{"hook_event_name":"PreToolUse","tool_name":"Grep","tool_input":{"pattern":"toolCall"}}', missing: true);

    expect(result.exitCode, 2);
    expect(result.stdout, isEmpty);
  });

  group('runs dashcrew guard with its arguments and stdin', () {
    const input = '{"tool_input": {"command": "echo \'a b\'"}}\nlast line without newline';
    final versions = ['0.1.0', '0.2.0', '1.0.0', '0.10.0', '0.1.1-0.20261101062238-4b1065d35c59', '0.1.0+abc', '12.0.3'];
    for (final (index, version) in versions.indexed) {
      final guardExit = 2 * (index % 2);
      test('$version, guard exit $guardExit', () async {
        final result = await runShim(input, version: version, guardExit: guardExit, args: ['/p/my policy.json', '--x', '']);

        expect(result.exitCode, guardExit, reason: '${result.stderr}');
        expect(recorded('args'), '[/p/my policy.json][--x][]');
        expect(recorded('stdin'), input);
        expect(result.stdout, 'guard-out\n');
        expect(result.stderr, 'guard-err\n');
      });
    }
  });

  group('version cache', () {
    int versionCalls() {
      final file = File(p.join(root.path, 'versions'));
      return file.existsSync() ? '\n'.allMatches(file.readAsStringSync()).length : 0;
    }

    void writeCache(String line) {
      final file = File(cachePath);
      file.parent.createSync(recursive: true);
      if (file.existsSync()) Process.runSync('chmod', ['644', cachePath]);
      file.writeAsStringSync(line);
    }

    Future<ProcessResult> cached() => runShim(claudePush, version: '0.2.0', args: ['p.json'], keepCache: true);

    test('a miss runs --version and writes the line, a hit skips it', () async {
      final standIn = p.join(bin, 'dashcrew');

      final miss = await runShim(claudePush, version: '0.2.0', args: ['p.json']);

      expect(miss.exitCode, 0);
      expect(versionCalls(), 1);
      expect(File(cachePath).readAsStringSync(), '$standIn 0.1.0\n');

      final hit = await cached();

      expect(hit.exitCode, 0);
      expect(versionCalls(), 0);
      expect(recorded('args'), '[p.json]');
      expect(recorded('stdin'), claudePush);
    });

    final misses = {
      'another path': (String standIn) => '${p.join('/elsewhere', 'dashcrew')} 0.1.0\n',
      'another need': (String standIn) => '$standIn 0.0.9\n',
      'corrupt': (String standIn) => 'garbage \u0000',
      'empty': (String standIn) => '',
      'no newline': (String standIn) => '$standIn 0.1.0',
    };
    for (final MapEntry(key: name, value: line) in misses.entries) {
      test('$name falls back to the full check', () async {
        writeCache(line(p.join(bin, 'dashcrew')));

        final result = await cached();

        expect(result.exitCode, 0);
        expect(versionCalls(), 1);
      });
    }

    test('an unreadable cache falls back to the full check', () async {
      writeCache('${p.join(bin, 'dashcrew')} 0.1.0\n');
      Process.runSync('chmod', ['000', cachePath]);

      final result = await cached();

      expect(result.exitCode, 0);
      expect(versionCalls(), 1);
    });

    test('a binary newer than the cache falls back to the full check', () async {
      writeCache('${p.join(bin, 'dashcrew')} 0.1.0\n');
      File(p.join(bin, 'dashcrew')).setLastModifiedSync(DateTime.now().add(const Duration(hours: 1)));

      final result = await cached();

      expect(result.exitCode, 0);
      expect(versionCalls(), 1);
    });

    test('a refusal leaves the cache as it was', () async {
      writeCache('${p.join(bin, 'dashcrew')} 0.0.9\n');

      final result = await runShim(claudePush, version: '0.0.1', keepCache: true);

      expect(result.exitCode, 2);
      expect(recorded('args'), isNull);
      expect(File(cachePath).readAsStringSync(), '${p.join(bin, 'dashcrew')} 0.0.9\n');
    });
  });
}
