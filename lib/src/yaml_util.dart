import 'package:yaml/yaml.dart';

/// Converts parsed YAML nodes into plain Dart maps and lists.
Object? plainYaml(Object? node) {
  if (node is YamlMap) {
    return {for (final entry in node.entries) entry.key.toString(): plainYaml(entry.value)};
  }
  if (node is YamlList) return [for (final item in node) plainYaml(item)];
  return node;
}

/// Looks up a dotted path (`policy.agent_commit`) in nested maps; null when absent.
Object? lookupPath(Map<String, Object?> context, String dottedPath) {
  Object? current = context;
  for (final segment in dottedPath.split('.')) {
    if (current is! Map) return null;
    current = current[segment];
  }
  return current;
}
