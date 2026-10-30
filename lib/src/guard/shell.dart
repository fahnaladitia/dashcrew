/// Simple commands in a shell script, as argv words; quoted text and heredoc bodies are data.
/// Commands inside `$(...)` and backticks are returned too, also when they sit inside double quotes.
/// A heredoc body is parsed as commands when its line runs a shell (`bash <<EOF`, `cat <<EOF | sh`).
List<List<String>> shellCommands(String script) => _ShellScanner(script).scan();

const _shellReceivers = {'sh', 'bash', 'zsh', 'dash', 'ksh', 'ssh', 'eval', 'source', '.'};

class _ShellScanner {
  _ShellScanner(this.source);

  final String source;
  int _pos = 0;
  final _commands = <List<String>>[];
  final _substitutions = <List<String>>[];
  var _current = <String>[];
  final _word = StringBuffer();
  var _inWord = false;
  final _pendingHeredocs = <({String delimiter, bool stripTabs})>[];
  var _lineStart = 0;

  List<List<String>> scan() {
    while (_pos < source.length) {
      final c = source[_pos];
      if (c == '\n') {
        _endCommand();
        final feedsShell = _commands.skip(_lineStart).any((argv) => argv.any((w) => _shellReceivers.contains(w.split('/').last)));
        _pos++;
        _skipHeredocBodies(parse: feedsShell);
        _lineStart = _commands.length;
      } else if (c == ' ' || c == '\t') {
        _endWord();
        _pos++;
      } else if (c == '#' && !_inWord) {
        while (_pos < source.length && source[_pos] != '\n') {
          _pos++;
        }
      } else if (c == ';' || c == '(' || c == ')') {
        _endCommand();
        _pos++;
      } else if (c == '&' || c == '|') {
        _endCommand();
        _pos++;
        if (_pos < source.length && (source[_pos] == c || source[_pos] == '&')) _pos++;
      } else if (c == '<' || c == '>') {
        _redirect();
      } else if (c == '\\') {
        _inWord = true;
        if (_pos + 1 < source.length && source[_pos + 1] != '\n') _word.write(source[_pos + 1]);
        _pos += 2;
      } else if (c == "'") {
        _inWord = true;
        final end = source.indexOf("'", _pos + 1);
        final stop = end < 0 ? source.length : end;
        _word.write(source.substring(_pos + 1, stop));
        _pos = stop + 1;
      } else if (c == '"') {
        _inWord = true;
        _doubleQuoted();
      } else if (c == r'$' && _peek(1) == '(') {
        _inWord = true;
        _substitution(_pos + 2, ')');
      } else if (c == '`') {
        _inWord = true;
        _substitution(_pos + 1, '`');
      } else {
        _inWord = true;
        _word.write(c);
        _pos++;
      }
    }
    _endCommand();
    return [..._commands, ..._substitutions];
  }

  String? _peek(int offset) => _pos + offset < source.length ? source[_pos + offset] : null;

  void _endWord() {
    if (_inWord) _current.add(_word.toString());
    _word.clear();
    _inWord = false;
  }

  void _endCommand() {
    _endWord();
    if (_current.isNotEmpty) _commands.add(_current);
    _current = <String>[];
  }

  void _doubleQuoted() {
    _pos++;
    while (_pos < source.length && source[_pos] != '"') {
      final c = source[_pos];
      if (c == '\\' && _pos + 1 < source.length) {
        _word.write(source[_pos + 1]);
        _pos += 2;
      } else if (c == r'$' && _peek(1) == '(') {
        _substitution(_pos + 2, ')');
      } else if (c == '`') {
        _substitution(_pos + 1, '`');
      } else {
        _word.write(c);
        _pos++;
      }
    }
    _pos++;
  }

  /// Scans a `$(...)` or backtick body starting at [start], recursing into it.
  void _substitution(int start, String close) {
    var depth = 0;
    var i = start;
    while (i < source.length) {
      final c = source[i];
      if (c == '\\') {
        i += 2;
        continue;
      }
      if (c == "'" && close == ')') {
        final end = source.indexOf("'", i + 1);
        i = end < 0 ? source.length : end + 1;
        continue;
      }
      if (close == ')' && c == '(') depth++;
      if (c == close) {
        if (depth == 0) break;
        depth--;
      }
      i++;
    }
    _substitutions.addAll(shellCommands(source.substring(start, i.clamp(start, source.length))));
    _word.write('<subst>');
    _pos = i + 1;
  }

  void _redirect() {
    final pending = _word.toString();
    if (_inWord && RegExp(r'^\d+$').hasMatch(pending)) {
      _word.clear();
      _inWord = false;
    } else {
      _endWord();
    }

    if (source.startsWith('<<<', _pos)) {
      _pos += 3;
      _skipTargetWord();
      return;
    }
    if (source.startsWith('<<', _pos)) {
      _pos += 2;
      var stripTabs = false;
      if (_peek(0) == '-') {
        stripTabs = true;
        _pos++;
      }
      final delimiter = _readTargetWord();
      if (delimiter.isNotEmpty) _pendingHeredocs.add((delimiter: delimiter, stripTabs: stripTabs));
      return;
    }

    _pos++;
    while (_pos < source.length && '<>&|'.contains(source[_pos])) {
      _pos++;
    }
    if (_pos < source.length && RegExp(r'[\d-]').hasMatch(source[_pos]) && '&>'.contains(source[_pos - 1])) {
      while (_pos < source.length && RegExp(r'[\d-]').hasMatch(source[_pos])) {
        _pos++;
      }
      return;
    }
    _skipTargetWord();
  }

  void _skipTargetWord() => _readTargetWord();

  String _readTargetWord() {
    while (_pos < source.length && (source[_pos] == ' ' || source[_pos] == '\t')) {
      _pos++;
    }
    final buffer = StringBuffer();
    while (_pos < source.length && !' \t\n;&|()<>'.contains(source[_pos])) {
      final c = source[_pos];
      if (c == "'" || c == '"') {
        final end = source.indexOf(c, _pos + 1);
        final stop = end < 0 ? source.length : end;
        buffer.write(source.substring(_pos + 1, stop));
        _pos = stop + 1;
      } else if (c == '\\' && _pos + 1 < source.length) {
        buffer.write(source[_pos + 1]);
        _pos += 2;
      } else {
        buffer.write(c);
        _pos++;
      }
    }
    return buffer.toString();
  }

  /// Moves past pending heredoc bodies; with [parse] their lines are scanned as commands instead of dropped.
  void _skipHeredocBodies({required bool parse}) {
    for (final heredoc in _pendingHeredocs) {
      final body = <String>[];
      while (_pos < source.length) {
        final end = source.indexOf('\n', _pos);
        final stop = end < 0 ? source.length : end;
        var line = source.substring(_pos, stop);
        if (heredoc.stripTabs) line = line.replaceFirst(RegExp(r'^\t+'), '');
        _pos = stop + 1;
        if (line == heredoc.delimiter) break;
        body.add(line);
      }
      if (parse) _substitutions.addAll(shellCommands(body.join('\n')));
    }
    _pendingHeredocs.clear();
  }
}
