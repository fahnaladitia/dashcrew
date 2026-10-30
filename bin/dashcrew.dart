import 'dart:io';

import 'package:args/command_runner.dart';

Future<void> main(List<String> arguments) async {
  final runner = CommandRunner<int>(
    'dashcrew',
    'Agent-agnostic AI coding toolkit for Flutter projects.',
  );
  try {
    exitCode = await runner.run(arguments) ?? 0;
  } on UsageException catch (error) {
    stderr.writeln(error);
    exitCode = 64;
  }
}
