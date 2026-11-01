import 'dart:convert';
import 'dart:io';

import 'package:dashcrew/dashcrew.dart';

/// Oracle for the Go shell parser's fuzz test: one JSON string per stdin line in, `shellCommands` as JSON out.
/// Build from the repository root: `dart compile exe internal/guard/shell/testdata/shell_oracle.dart -o <path>`.
Future<void> main() async {
  await for (final line in stdin.transform(utf8.decoder).transform(const LineSplitter())) {
    stdout.writeln(jsonEncode(shellCommands(jsonDecode(line) as String)));
  }
}
