import 'dart:io';

import 'package:args/command_runner.dart';
import 'package:path/path.dart' as p;
import 'package:dashcrew/dashcrew.dart';
import 'package:dashcrew/src/package_root.dart';

Future<void> main(List<String> arguments) async {
  final runner = CommandRunner<int>(
    'dashcrew',
    'Agent-agnostic AI coding toolkit for Flutter projects.',
  )
    ..addCommand(InitCommand())
    ..addCommand(SyncCommand())
    ..addCommand(DoctorCommand())
    ..addCommand(WorktreeCommand())
    ..addCommand(GuardCommand())
    ..addCommand(GateCommand());
  try {
    exitCode = await runner.run(arguments) ?? 0;
  } on UsageException catch (error) {
    stderr.writeln(error);
    exitCode = 64;
  } on SyncException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  } on InitException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  } on WorktreeException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  } on GuardInstallException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  } on GateException catch (error) {
    stderr.writeln(error.message);
    exitCode = 1;
  }
}

class InitCommand extends Command<int> {
  InitCommand() {
    argParser
      ..addOption('project', abbr: 'p', help: 'Flutter project root holding pubspec.yaml.', defaultsTo: '.')
      ..addFlag('yes', abbr: 'y', help: 'Write the detected profile without asking.', negatable: false)
      ..addMultiOption('agents', help: 'Agents to render for.', allowed: Profile.supportedAgents, defaultsTo: ['claude-code'])
      ..addFlag('force', help: 'Replace an existing .ai/project.yaml.', negatable: false);
  }

  @override
  String get name => 'init';

  @override
  String get description => 'Detect the stack from pubspec.yaml and write .ai/project.yaml. Writes nothing else.';

  @override
  int run() {
    final args = argResults!;
    final projectDir = p.normalize(Directory(args['project'] as String).absolute.path);
    Init(catalog: Catalog.load(dashcrewPackageRoot()), projectDir: projectDir, agents: args['agents'] as List<String>).run(
      yes: args['yes'] as bool,
      force: args['force'] as bool,
      remoteBranches: remoteBranches(projectDir),
      ask: (prompt) {
        stdout.write(prompt);
        return stdin.readLineSync();
      },
      out: stdout.writeln,
    );
    return 0;
  }
}

class SyncCommand extends Command<int> {
  SyncCommand() {
    argParser
      ..addOption('project', abbr: 'p', help: 'Project root holding .ai/project.yaml.', defaultsTo: '.')
      ..addFlag('force', help: 'Overwrite generated files that were edited by hand.', negatable: false)
      ..addFlag('adopt', help: 'Keep an existing AGENTS.md as the project block.', negatable: false)
      ..addFlag('compile-guard', help: 'Install the shared guard for this dashcrew when it is missing.', defaultsTo: true);
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

class DoctorCommand extends Command<int> {
  DoctorCommand() {
    argParser.addOption('project', abbr: 'p', help: 'Project root holding .ai/project.yaml.', defaultsTo: '.');
  }

  @override
  String get name => 'doctor';

  @override
  String get description => 'Check that the generated toolkit is installed, untouched and current. Writes nothing.';

  @override
  Future<int> run() async {
    final root = dashcrewPackageRoot();
    final checks = await Doctor(
      catalog: Catalog.load(root),
      projectDir: Directory(argResults!['project'] as String).absolute.path,
      packageRoot: root,
    ).run();

    for (final check in checks) {
      stdout.writeln('${check.status.name.padRight(6)}${check.id.padRight(16)}${check.detail}');
    }
    return checks.any((check) => check.status == CheckStatus.fail) ? 1 : 0;
  }
}

class WorktreeCommand extends Command<int> {
  WorktreeCommand() {
    addSubcommand(_WorktreeCreate());
    addSubcommand(_WorktreeList());
    addSubcommand(_WorktreeLink());
    addSubcommand(_WorktreeRemove());
    addSubcommand(_WorktreeWatch());
  }

  @override
  String get name => 'worktree';

  @override
  String get description => 'One worktree per parallel task. Never commits, pushes or deletes a branch.';
}

abstract class _WorktreeSubcommand extends Command<int> {
  _WorktreeSubcommand() {
    argParser.addOption('project', abbr: 'p', help: 'Any folder inside the repository.', defaultsTo: '.');
  }

  Worktrees get worktrees => Worktrees(projectDir: argResults!['project'] as String);

  String positional(int index, String label) {
    final rest = argResults!.rest;
    if (rest.length <= index) usageException('Missing <$label>.');
    return rest[index];
  }
}

class _WorktreeCreate extends _WorktreeSubcommand {
  _WorktreeCreate() {
    argParser
      ..addOption('name', help: 'Folder name; defaults to the branch with / replaced by -.')
      ..addFlag('pub', help: 'Run flutter pub get in the new worktree.', defaultsTo: true);
  }

  @override
  String get name => 'create';

  @override
  String get invocation => 'dashcrew worktree create <branch> <base>';

  @override
  String get description => 'Create <branch> from <base> in <repo>.worktrees/<name>, with no upstream and the toolkit linked.';

  @override
  int run() {
    worktrees.create(positional(0, 'branch'), positional(1, 'base'),
        name: argResults!['name'] as String?, pubGet: argResults!['pub'] as bool, out: stdout.writeln);
    return 0;
  }
}

class _WorktreeList extends _WorktreeSubcommand {
  @override
  String get name => 'list';

  @override
  String get description => 'Show each task worktree: branch, base, upstream, dirty lines, missing links.';

  @override
  int run() {
    final infos = worktrees.list();
    if (infos.isEmpty) stdout.writeln('no task worktrees');
    for (final info in infos) {
      final links = info.missingLinks.isEmpty ? 'links ok' : 'missing links: ${info.missingLinks.join(', ')}';
      stdout.writeln('${info.name}  ${info.branch}  base ${info.base ?? '-'}  upstream ${info.upstream ?? '-'}  dirty ${info.dirty}  $links');
    }
    return 0;
  }
}

class _WorktreeLink extends _WorktreeSubcommand {
  @override
  String get name => 'link';

  @override
  String get invocation => 'dashcrew worktree link <name>';

  @override
  String get description => 'Add missing toolkit links to a worktree and drop an upstream it should not have.';

  @override
  int run() {
    worktrees.link(positional(0, 'name'), out: stdout.writeln);
    return 0;
  }
}

class _WorktreeRemove extends _WorktreeSubcommand {
  @override
  String get name => 'remove';

  @override
  String get invocation => 'dashcrew worktree remove <name>';

  @override
  String get description => 'Remove a clean worktree. The branch is kept.';

  @override
  int run() {
    worktrees.remove(positional(0, 'name'), out: stdout.writeln);
    return 0;
  }
}

class _WorktreeWatch extends _WorktreeSubcommand {
  _WorktreeWatch() {
    argParser
      ..addFlag('once', help: 'Run one pass and exit.', negatable: false)
      ..addOption('state-dir', help: 'Where seen events are kept; defaults to <git dir>/dashcrew-watch.')
      ..addOption('interval', help: 'Seconds between passes.', defaultsTo: '60');
  }

  @override
  String get name => 'watch';

  @override
  String get invocation => 'dashcrew worktree watch [<name>...]';

  @override
  String get description => 'Print one line per new event (commit, pushed, pr, base-moved, merge-tree, overlap) until done.';

  @override
  Future<int> run() async {
    final worktrees = this.worktrees;
    final interval = int.tryParse(argResults!['interval'] as String);
    if (interval == null || interval < 1) usageException('--interval must be a positive number of seconds.');
    await Watch(
      worktrees: worktrees,
      stateDir: (argResults!['state-dir'] as String?) ?? p.join(worktrees.gitDir, 'dashcrew-watch'),
    ).run(argResults!.rest, once: argResults!['once'] as bool, interval: Duration(seconds: interval), out: stdout.writeln);
    return 0;
  }
}

class GuardCommand extends Command<int> {
  GuardCommand() {
    addSubcommand(_GuardInstall());
  }

  @override
  String get name => 'guard';

  @override
  String get description => 'The compiled guard every project on this machine shares.';
}

class _GuardInstall extends Command<int> {
  _GuardInstall() {
    argParser
      ..addOption('expect', help: 'Refuse unless this dashcrew builds the guard with this source hash (the lock\'s guard_source).')
      ..addFlag('force', help: 'Compile again even when installed.', negatable: false);
  }

  @override
  String get name => 'install';

  @override
  String get description => 'Compile the guard once into \$DASHCREW_HOME (default ~/.dashcrew) and print its path.';

  @override
  int run() {
    final result = GuardInstaller(packageRoot: dashcrewPackageRoot())
        .install(expect: argResults!['expect'] as String?, force: argResults!['force'] as bool);
    stdout.writeln(result.path);
    return 0;
  }
}

class GateCommand extends Command<int> {
  GateCommand() {
    addSubcommand(_GateSubmit());
    addSubcommand(_GateStatus());
  }

  @override
  String get name => 'gate';

  @override
  String get description => 'Human approval on the branch\'s GitHub issue. Approving happens on GitHub only.';
}

class _GateSubmit extends Command<int> {
  _GateSubmit() {
    argParser
      ..addOption('project', abbr: 'p', help: 'Any folder inside the repository.', defaultsTo: '.')
      ..addOption('file', help: 'The plan to post (required).')
      ..addOption('issue', help: 'Issue number to use; defaults to the branch\'s linked issue, else a new one.');
  }

  @override
  String get name => 'submit';

  @override
  String get invocation => 'dashcrew gate submit plan --file <plan.md> [--issue <n>]';

  @override
  String get description => 'Post the plan to the branch\'s GitHub issue, then stop and wait for /approve plan.';

  @override
  int run() {
    final rest = argResults!.rest;
    if (rest.length != 1 || rest.single != 'plan') usageException('The only gate is "plan": dashcrew gate submit plan --file <plan.md>.');
    final file = argResults!['file'] as String?;
    if (file == null) usageException('--file <plan.md> is required.');
    final issueText = argResults!['issue'] as String?;
    final issue = issueText == null ? null : int.tryParse(issueText);
    if (issueText != null && issue == null) usageException('--issue must be a number.');
    final result = Gates(projectDir: argResults!['project'] as String).submitPlan(file, issue: issue);
    stdout
      ..writeln('posted the plan to #${result.issue}: ${result.url}')
      ..writeln('waiting for a comment whose first line is "/approve plan"; run dashcrew gate status after it');
    return 0;
  }
}

class _GateStatus extends Command<int> {
  _GateStatus() {
    argParser.addOption('project', abbr: 'p', help: 'Any folder inside the repository.', defaultsTo: '.');
  }

  @override
  String get name => 'status';

  @override
  String get description => 'Read the branch\'s plan issue and record whether the latest plan is approved.';

  @override
  int run() {
    final state = Gates(projectDir: argResults!['project'] as String).status();
    stdout.writeln(switch (state.plan) {
      'approved' => 'plan: approved (#${state.issue})',
      'pending' => 'plan: pending (#${state.issue}); waiting for "/approve plan" after the latest plan',
      _ => 'plan: no issue linked to this branch; dashcrew gate submit plan --file <plan.md> starts one',
    });
    return 0;
  }
}
