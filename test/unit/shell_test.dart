import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';

List<List<String>> commands(String script) => shellCommands(script);

void main() {
  test('splits on separators and keeps argv words', () {
    expect(commands('git status && flutter test test/a_test.dart; ls | wc -l'), [
      ['git', 'status'],
      ['flutter', 'test', 'test/a_test.dart'],
      ['ls'],
      ['wc', '-l'],
    ]);
  });

  test('quoted text is one data word, never a command', () {
    expect(commands('grep -rn "git commit" docs/'), [
      ['grep', '-rn', 'git commit', 'docs/'],
    ]);
    expect(commands("echo 'a; git push'"), [
      ['echo', 'a; git push'],
    ]);
  });

  test('command substitution is a command, also inside double quotes', () {
    expect(commands(r'echo "$(git push origin)"'), anyElement(equals(['git', 'push', 'origin'])));
    expect(commands('x=`git stash`'), anyElement(equals(['git', 'stash'])));
  });

  test('heredoc body is data', () {
    const script = 'cat > notes.md <<\'EOF\'\nrun git commit later\ndart format is banned\nEOF\nls';
    expect(commands(script), [
      ['cat'],
      ['ls'],
    ]);
  });

  test('unquoted heredoc body is data too', () {
    const script = 'cat <<-EOF > a.md\n\tgit push\n\tEOF\necho done';
    expect(commands(script), [
      ['cat'],
      ['echo', 'done'],
    ]);
  });

  test('heredoc fed to a shell on its line is parsed as commands', () {
    expect(commands('bash <<EOF\ngit push\nEOF\nls'), containsAll([
      ['bash'],
      ['git', 'push'],
      ['ls'],
    ]));
    expect(commands("cat <<'EOF' | /bin/sh\ndart format lib\nEOF"), anyElement(equals(['dart', 'format', 'lib'])));
  });

  test('redirect targets are not argv words', () {
    expect(commands('flutter test > out.txt 2>&1'), [
      ['flutter', 'test'],
    ]);
  });

  test('subshell parentheses split commands', () {
    expect(commands('(cd app && git commit -m x)'), [
      ['cd', 'app'],
      ['git', 'commit', '-m', 'x'],
    ]);
  });

  test('escaped characters stay in the word', () {
    expect(commands(r'echo a\;b'), [
      ['echo', 'a;b'],
    ]);
  });
}
