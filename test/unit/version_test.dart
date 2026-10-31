import 'dart:io';

import 'package:dashcrew/dashcrew.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  test('dashcrewVersion matches pubspec.yaml', () {
    final pubspec = loadYaml(File('pubspec.yaml').readAsStringSync()) as YamlMap;

    expect(dashcrewVersion, pubspec['version']);
  });

  test('compareVersions orders by major, minor, patch and ignores build metadata', () {
    expect(compareVersions('0.1.0', '0.0.9'), greaterThan(0));
    expect(compareVersions('1.2.3', '1.10.0'), lessThan(0));
    expect(compareVersions('1.2.3+4', '1.2.3'), 0);
    expect(compareVersions('2.0.0-dev.1', '2.0.0'), lessThan(0));
  });
}
