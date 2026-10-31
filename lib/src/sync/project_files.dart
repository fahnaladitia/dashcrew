import 'dart:io';

import 'package:path/path.dart' as p;

/// Files of one project, addressed by paths relative to its root.
/// Never writes or deletes through a symbolic link inside the project: the link is replaced, its target is left alone.
class ProjectFiles {
  const ProjectFiles(this.root);

  final String root;

  File file(String relative) => File(p.join(root, relative));

  /// The first part of [relative] that is a symbolic link, with the link's target; null when no part is one.
  ({String path, String target})? linkWithin(String relative) {
    var current = root;
    for (final part in p.split(relative)) {
      current = p.join(current, part);
      if (FileSystemEntity.isLinkSync(current)) {
        return (path: p.relative(current, from: root), target: Link(current).targetSync());
      }
      if (!FileSystemEntity.isDirectorySync(current)) return null;
    }
    return null;
  }

  /// Writes [content] unless the file already holds it; true when it wrote.
  /// A link on the way is removed first, so the write lands in the project and never in the link's target.
  bool writeIfChanged(String relative, String content) {
    final target = file(relative);
    final link = linkWithin(relative);
    if (link == null && target.existsSync() && target.readAsStringSync() == content) return false;
    if (link != null) Link(p.join(root, link.path)).deleteSync();
    target.parent.createSync(recursive: true);
    target.writeAsStringSync(content);
    return true;
  }
}
