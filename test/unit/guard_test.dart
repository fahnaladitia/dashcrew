import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

GuardPolicy policy({bool agentCommit = false, bool formatter = false}) => GuardPolicy(
      blockedGit: {...GuardPolicy.alwaysBlockedGit, if (!agentCommit) 'commit'},
      blockFormatters: !formatter,
      plansDir: '.ai/plans',
      roleCapability: const {
        'flutter-planner': 'write-plans',
        'flutter-code-reviewer': 'read',
        'flutter-engineer': 'write-code',
      },
      forbiddenImports: const ['package:vendor_sdk/'],
      localPackages: const ['core'],
    );

GuardEvent shell(String command, {String? role}) => GuardEvent(kind: ToolKind.shell, command: command, role: role);

GuardEvent write(String path, {String content = 'x', String? role, ToolKind kind = ToolKind.write}) =>
    GuardEvent(kind: kind, filePath: path, content: content, role: role);

void main() {
  final blockedShell = {
    'git commit -m x': 'commit',
    'git push': 'push',
    'rtk git push origin main': 'rtk wrapper',
    'sudo git stash': 'sudo wrapper',
    'FOO=1 git reset --hard': 'env assignment',
    'git -C app commit -m x': 'global -C option',
    'git -c user.name=a commit': 'global -c option',
    'git --no-pager rebase main': 'global --no-pager',
    'ls && git merge dev': 'after &&',
    r'echo "$(git push)"': 'substitution in quotes',
    'dart format lib/': 'formatter',
    'dart fix --apply': 'dart fix',
    'flutter format lib/': 'flutter format',
    'git tag v1': 'tag',
    'git cherry-pick abc': 'cherry-pick',
    'git revert abc': 'revert',
    'bash <<EOF\ngit push\nEOF': 'heredoc run by bash',
    "ssh host <<'EOF'\ngit push\nEOF": 'heredoc run over ssh',
    'cat <<EOF | sh\ngit commit -m x\nEOF': 'heredoc piped into sh',
  };
  blockedShell.forEach((command, why) {
    test('blocks: $why ($command)', () {
      expect(evaluate(shell(command), policy()).blocked, isTrue);
    });
  });

  final allowedShell = {
    'git status': 'read-only git',
    'git diff --stat': 'diff',
    'git log --oneline dev..HEAD': 'log',
    'grep -rn "git commit" docs/': 'quoted text',
    'cat > a.md <<\'EOF\'\nnever git push here\nEOF': 'heredoc body',
    "python3 - <<'PY'\nprint('git push')\nPY": 'heredoc read by python',
    'dart analyze lib/': 'analyze',
    'flutter test test/a_test.dart': 'test',
    'git checkout --no-track -b feat/x main': 'branch creation',
    'echo commit': 'word commit alone',
  };
  allowedShell.forEach((command, why) {
    test('allows: $why ($command)', () {
      expect(evaluate(shell(command), policy()).blocked, isFalse);
    });
  });

  test('agent_commit true allows commit but still blocks push', () {
    final p = policy(agentCommit: true);
    expect(evaluate(shell('git commit -m x'), p).blocked, isFalse);
    expect(evaluate(shell('git push'), p).blocked, isTrue);
  });

  test('formatter true allows dart format', () {
    expect(evaluate(shell('dart format lib/'), policy(formatter: true)).blocked, isFalse);
  });

  test('block reason names the command', () {
    expect(evaluate(shell('git -C app push'), policy()).reason, contains('git push'));
  });

  test('write-plans role may write under the plans dir only', () {
    expect(evaluate(write('/repo/.ai/plans/orders/plan.md', role: 'flutter-planner'), policy()).blocked, isFalse);
    expect(evaluate(write('/repo/lib/main.dart', role: 'flutter-planner'), policy()).blocked, isTrue);
    expect(
      evaluate(write('/repo/.ai/plans/orders/plan.md', role: 'flutter-planner', kind: ToolKind.edit), policy()).blocked,
      isTrue,
    );
  });

  test('read role cannot write or edit', () {
    expect(evaluate(write('/repo/.ai/plans/x.md', role: 'flutter-code-reviewer'), policy()).blocked, isTrue);
    expect(evaluate(write('/repo/lib/a.dart', role: 'flutter-code-reviewer', kind: ToolKind.edit), policy()).blocked, isTrue);
  });

  test('write-code role and the main session write freely', () {
    expect(evaluate(write('/repo/lib/a.dart', role: 'flutter-engineer'), policy()).blocked, isFalse);
    expect(evaluate(write('/repo/lib/a.dart'), policy()).blocked, isFalse);
  });

  test('unknown role is not restricted', () {
    expect(evaluate(write('/repo/lib/a.dart', role: 'Explore'), policy()).blocked, isFalse);
  });

  test('forbidden import is blocked in app lib only', () {
    const content = "import 'package:vendor_sdk/vendor_sdk.dart';\n";
    expect(evaluate(write('/repo/lib/feature/a.dart', content: content), policy()).blocked, isTrue);
    expect(evaluate(write('/repo/core/lib/a.dart', content: content), policy()).blocked, isFalse);
    expect(evaluate(write('/repo/test/a_test.dart', content: content), policy()).blocked, isFalse);
  });

  group('GuardEvent.fromHook', () {
    test('claude code bash and write input', () {
      final bash = GuardEvent.fromHook({
        'tool_name': 'Bash',
        'tool_input': {'command': 'git push'},
        'agent_type': 'flutter-planner',
      });
      expect(bash.kind, ToolKind.shell);
      expect(bash.command, 'git push');
      expect(bash.role, 'flutter-planner');

      final edit = GuardEvent.fromHook({
        'tool_name': 'Edit',
        'tool_input': {'file_path': '/repo/lib/a.dart', 'new_string': 'y'},
      });
      expect(edit.kind, ToolKind.edit);
      expect(edit.filePath, '/repo/lib/a.dart');
      expect(edit.content, 'y');
    });

    test('codex shell input as argv list unwraps bash -lc', () {
      final event = GuardEvent.fromHook({
        'tool_name': 'shell',
        'tool_input': {
          'command': ['bash', '-lc', 'git push origin main'],
        },
      });
      expect(event.kind, ToolKind.shell);
      expect(event.command, 'git push origin main');
    });

    test('unknown tool is ignored', () {
      expect(GuardEvent.fromHook({'tool_name': 'WebFetch', 'tool_input': {}}).kind, ToolKind.other);
    });
  });

  test('policy survives a json round trip', () {
    final original = policy();
    final copy = GuardPolicy.fromJson(original.toJson());
    expect(copy.toJson(), original.toJson());
  });
}
