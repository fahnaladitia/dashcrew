import 'dart:convert';

import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

GuardPolicy policy({bool agentCommit = false, bool formatter = false}) => GuardPolicy(
      blockedGit: {...GuardPolicy.alwaysBlockedGit, if (!agentCommit) 'commit'},
      blockFormatters: !formatter,
      plansDir: '.ai/plans',
      roleCapability: const {
        'planner': 'write-plans',
        'reviewer': 'read',
        'engineer': 'write-code',
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
    expect(evaluate(write('/repo/.ai/plans/orders/plan.md', role: 'planner'), policy()).blocked, isFalse);
    expect(evaluate(write('/repo/lib/main.dart', role: 'planner'), policy()).blocked, isTrue);
    expect(
      evaluate(write('/repo/.ai/plans/orders/plan.md', role: 'planner', kind: ToolKind.edit), policy()).blocked,
      isTrue,
    );
  });

  test('read role cannot write or edit', () {
    expect(evaluate(write('/repo/.ai/plans/x.md', role: 'reviewer'), policy()).blocked, isTrue);
    expect(evaluate(write('/repo/lib/a.dart', role: 'reviewer', kind: ToolKind.edit), policy()).blocked, isTrue);
  });

  test('write-code role and the main session write freely', () {
    expect(evaluate(write('/repo/lib/a.dart', role: 'engineer'), policy()).blocked, isFalse);
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
        'agent_type': 'planner',
      });
      expect(bash.kind, ToolKind.shell);
      expect(bash.command, 'git push');
      expect(bash.role, 'planner');

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

  group('codex apply_patch', () {
    List<GuardEvent> patch(String text) => GuardEvent.allFromHook({
          'tool_name': 'apply_patch',
          'tool_input': {'command': text},
        });

    bool blocked(String text) => evaluateAll(patch(text), policy()).blocked;

    test('one event per file, with added lines as content', () {
      final events = patch('''
*** Begin Patch
*** Add File: test/a_test.dart
+void main() {}
*** Update File: lib/b.dart
@@
-old()
+fresh()
*** Delete File: lib/c.dart
*** End Patch
''');

      expect(events.map((e) => e.filePath), ['test/a_test.dart', 'lib/b.dart', 'lib/c.dart']);
      expect(events.map((e) => e.kind), [ToolKind.write, ToolKind.edit, ToolKind.edit]);
      expect(events[1].content, 'fresh()');
    });

    test('a forbidden import in the second file blocks the patch', () {
      expect(blocked('''
*** Begin Patch
*** Update File: test/a_test.dart
+import 'package:vendor_sdk/vendor_sdk.dart';
*** Update File: lib/feature/b.dart
+import 'package:vendor_sdk/vendor_sdk.dart';
*** End Patch
'''), isTrue);
    });

    test('removing a forbidden import is allowed', () {
      expect(blocked('''
*** Begin Patch
*** Update File: lib/feature/b.dart
-import 'package:vendor_sdk/vendor_sdk.dart';
+import 'package:app/analytics.dart';
*** End Patch
'''), isFalse);
    });

    test('a move into lib is checked at its new path', () {
      expect(blocked('''
*** Begin Patch
*** Update File: test/x.dart
*** Move to: lib/x.dart
+import 'package:vendor_sdk/vendor_sdk.dart';
*** End Patch
'''), isTrue);
    });

    test('codex Bash input is one shell event', () {
      final events = GuardEvent.allFromHook({
        'tool_name': 'Bash',
        'tool_input': {'command': 'git push'},
      });

      expect(events.single.kind, ToolKind.shell);
      expect(evaluateAll(events, policy()).blocked, isTrue);
    });
  });

  group('antigravity toolCall input', () {
    List<GuardEvent> events(String name, Map<String, Object?> args) => GuardEvent.allFromHook({
          'conversationId': 'c1',
          'workspacePaths': ['/repo'],
          'toolCall': {'name': name, 'args': args},
        });

    bool blocked(String name, Map<String, Object?> args) => evaluateAll(events(name, args), policy()).blocked;

    test('run_command is a shell event', () {
      expect(events('run_command', {'CommandLine': 'git push', 'Cwd': '/repo'}).single.kind, ToolKind.shell);
      expect(blocked('run_command', {'CommandLine': 'git push'}), isTrue);
      expect(blocked('run_command', {'CommandLine': 'git status'}), isFalse);
    });

    test('write_to_file and replace_file_content carry the path and new text', () {
      const import = "import 'package:vendor_sdk/vendor_sdk.dart';";
      expect(blocked('write_to_file', {'TargetFile': 'lib/feature/a.dart', 'CodeContent': import}), isTrue);
      expect(blocked('replace_file_content', {'TargetFile': 'lib/feature/a.dart', 'TargetContent': 'x', 'ReplacementContent': import}), isTrue);
      expect(blocked('replace_file_content', {'TargetFile': 'lib/feature/a.dart', 'TargetContent': import, 'ReplacementContent': ''}), isFalse);
      expect(events('write_to_file', {'TargetFile': 'a.dart', 'CodeContent': ''}).single.kind, ToolKind.write);
    });

    test('multi_replace_file_content checks only replacement text of every chunk', () {
      const import = "import 'package:vendor_sdk/vendor_sdk.dart';";
      expect(
        blocked('multi_replace_file_content', {
          'TargetFile': 'lib/feature/a.dart',
          'ReplacementChunks': [
            {'TargetContent': 'a', 'ReplacementContent': 'b'},
            {'TargetContent': 'c', 'ReplacementContent': import},
          ],
        }),
        isTrue,
      );
      expect(
        blocked('multi_replace_file_content', {
          'TargetFile': 'lib/feature/a.dart',
          'ReplacementChunks': [
            {'TargetContent': import, 'ReplacementContent': ''},
          ],
        }),
        isFalse,
      );
    });

    test('other tools and missing args are allowed', () {
      expect(events('view_file', {'AbsolutePath': '/repo/a'}).single.kind, ToolKind.other);
      expect(blocked('run_command', {}), isFalse);
    });
  });

  group('hookResponse', () {
    const block = GuardDecision.block('git push is reserved for the user. Ask them to run it.');

    test('claude code and codex get exit 2 and the reason on stderr', () {
      final response = hookResponse({'tool_name': 'Bash'}, block);

      expect(response.exitCode, 2);
      expect(response.stderr, '[dashcrew guard] git push is reserved for the user. Ask them to run it.');
      expect(response.stdout, isEmpty);
    });

    test('antigravity gets a deny decision on stdout and exit 0', () {
      final response = hookResponse({'toolCall': {'name': 'run_command'}}, block);

      expect(response.exitCode, 0);
      expect(jsonDecode(response.stdout), {
        'decision': 'deny',
        'reason': '[dashcrew guard] git push is reserved for the user. Ask them to run it.',
      });
    });

    test('an allowed call prints nothing for every agent', () {
      for (final input in [
        {'tool_name': 'Bash'},
        {'toolCall': {'name': 'run_command'}},
      ]) {
        final response = hookResponse(input, const GuardDecision.allow());
        expect((response.exitCode, response.stdout, response.stderr), (0, '', ''));
      }
    });
  });

  test('an older guard.json that still carries plan_gate loads, and the key is dropped', () {
    final loaded = GuardPolicy.fromJson({...policy().toJson(), 'plan_gate': true});

    expect(loaded.toJson(), isNot(contains('plan_gate')));
    expect(evaluate(write('/repo/lib/a.dart'), loaded).blocked, isFalse);
    expect(evaluate(shell('gh issue comment 42 --body "/approve plan"'), loaded).blocked, isTrue);
  });

  group('approvals are the user\'s', () {
    for (final command in [
      'gh issue comment 42 --body "/approve plan"',
      "gh api repos/a/b/issues/42/comments -f body='/approve plan'",
      'rtk gh pr comment 3 --body "/approve plan"',
    ]) {
      test('blocks $command', () {
        final decision = evaluate(shell(command), policy());
        expect(decision.blocked, isTrue);
        expect(decision.reason, contains("Approvals are the user's"));
        expect(decision.reason, isNot(contains('dashcrew gate')));
      });
    }

    test('reading issues and mentioning approve elsewhere is fine', () {
      expect(evaluate(shell('gh issue view 42'), policy()).blocked, isFalse);
      expect(evaluate(shell('echo "/approve plan"'), policy()).blocked, isFalse);
    });
  });

  test('policy survives a json round trip', () {
    final original = policy();
    final copy = GuardPolicy.fromJson(original.toJson());
    expect(copy.toJson(), original.toJson());
  });
}
