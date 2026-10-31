import 'dart:io';

import 'package:path/path.dart' as p;

/// Files of one project, addressed by paths relative to its root.
class ProjectFiles {
  const ProjectFiles(this.root);

  final String root;

  File file(String relative) => File(p.join(root, relative));

  /// Writes [content] unless the file already holds it; true when it wrote.
  bool writeIfChanged(String relative, String content) {
    final target = file(relative);
    if (target.existsSync() && target.readAsStringSync() == content) return false;
    target.parent.createSync(recursive: true);
    target.writeAsStringSync(content);
    return true;
  }
}
