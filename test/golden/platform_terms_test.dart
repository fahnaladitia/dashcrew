import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:test/test.dart';

/// Flutter and Dart terms that must not reach a project on another platform.
final _terms = RegExp(r'flutter|dart|pubspec|widget|semantics|buildcontext', caseSensitive: false);

void main() {
  test('sample-other renders no Flutter or Dart term in any file', () {
    final expected = Directory(p.join('test', 'golden', 'sample-other', 'expected'));
    final found = <String>[];
    for (final file in expected.listSync(recursive: true).whereType<File>()) {
      final path = p.relative(file.path, from: expected.path).replaceAll(r'\', '/');
      final lines = file.readAsStringSync().split('\n');
      for (var i = 0; i < lines.length; i++) {
        for (final match in _terms.allMatches(lines[i])) {
          found.add('$path:${i + 1}: ${match.group(0)}');
        }
      }
    }

    expect(found, isEmpty);
  });
}
