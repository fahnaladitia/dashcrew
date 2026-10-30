import 'dart:io';

import 'package:args/command_runner.dart';
import 'package:dashcrew/dashcrew.dart';
import 'package:dashcrew/src/package_root.dart';

Future<void> main(List<String> arguments) async {
  final runner = CommandRunner<int>(
    'dashcrew',
    'Agent-agnostic AI coding toolkit for Flutter projects.',
  )..addCommand(SyncCommand());
  try {
    exitCode = await runner.run(arguments) ?? 0;
  } on UsageException catch (error) {
    stderr.writeln(error);
    exitCode = 64;
  } on SyncException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  }
}

class SyncCommand extends Command<int> {
  SyncCommand() {
    argParser
      ..addOption('project', abbr: 'p', help: 'Project root holding .ai/project.yaml.', defaultsTo: '.')
      ..addFlag('force', help: 'Overwrite generated files that were edited by hand.', negatable: false)
      ..addFlag('adopt', help: 'Keep an existing AGENTS.md as the project block.', negatable: false)
      ..addFlag('compile-guard', help: 'Compile .ai/bin/guard when its sources changed.', defaultsTo: true);
  }

  @override
  String get name => 'sync';

  @override
  String get description => 'Render rules, roles, skills, guard and hooks for the agents in .ai/project.yaml.';

  @override
  int run() {
    final args = argResults!;
    final root = dashcrewPackageRoot();
    final result = Sync(
      catalog: Catalog.load(root),
      projectDir: Directory(args['project'] as String).absolute.path,
      compileGuard: args['compile-guard'] as bool,
      packageRoot: root,
    ).run(force: args['force'] as bool, adopt: args['adopt'] as bool);

    for (final path in result.written) {
      stdout.writeln('wrote   $path');
    }
    for (final path in result.removed) {
      stdout.writeln('removed $path');
    }
    for (final warning in result.warnings) {
      stderr.writeln('warning: $warning');
    }
    if (result.written.isEmpty && result.removed.isEmpty) stdout.writeln('up to date');
    return 0;
  }
}
