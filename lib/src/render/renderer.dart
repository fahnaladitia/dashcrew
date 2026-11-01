import 'package:mustache_template/mustache_template.dart';

import '../catalog/catalog.dart';
import '../profile/profile.dart';
import '../yaml_util.dart';

/// Template error, with the fragment source that failed.
class RenderException implements Exception {
  RenderException(this.source, this.message);

  final String source;
  final String message;

  @override
  String toString() => '$source: $message';
}

/// Rendered content for one target, plus the rule ids it covers.
class RenderedTarget {
  const RenderedTarget({required this.target, required this.content, required this.rules});

  final String target;
  final String content;
  final Set<String> rules;
}

/// Renders the catalog for one profile: core and the profile's platform plus selected packs, filtered by `when`.
/// Templates are strict; an unknown variable fails instead of rendering empty.
class Renderer {
  const Renderer(this.catalog);

  final Catalog catalog;

  List<RenderedTarget> render(Profile profile) {
    final context = contextFor(profile);
    final fragments = [
      ...catalog.fragmentsFor(profile.platform),
      for (final pack in selectedPacks(profile)) ...pack.fragments,
    ];

    final byTarget = <String, List<Fragment>>{};
    for (final fragment in fragments) {
      if (whenMatches(fragment.when, context)) (byTarget[fragment.target] ??= []).add(fragment);
    }
    context['skill_index'] = _skillIndex(profile, byTarget.keys);

    final targets = byTarget.keys.toList()..sort();
    return [
      for (final target in targets) _renderTarget(target, byTarget[target]!, context),
    ];
  }

  RenderedTarget _renderTarget(String target, List<Fragment> fragments, Map<String, Object?> context) {
    fragments.sort((a, b) {
      final byOrder = a.order.compareTo(b.order);
      return byOrder != 0 ? byOrder : a.source.compareTo(b.source);
    });
    final parts = [for (final fragment in fragments) _renderFragment(fragment, context).trimRight()];
    return RenderedTarget(
      target: target,
      content: '${parts.join('\n\n')}\n',
      rules: {for (final fragment in fragments) ...fragment.rules},
    );
  }

  String _renderFragment(Fragment fragment, Map<String, Object?> context) {
    try {
      return Template(fragment.body, name: fragment.source, lenient: false, htmlEscapeValues: false)
          .renderString(context);
    } on TemplateException catch (error) {
      throw RenderException(fragment.source, error.message);
    }
  }

  /// `skill_index`: one entry per skill target that renders and has a description, sorted by name.
  /// A skill without `when_to_use` is indexed by its description.
  List<Map<String, Object?>> _skillIndex(Profile profile, Iterable<String> targets) {
    final metas = {for (final pack in selectedPacks(profile)) ...pack.skills, ...catalog.skillsFor(profile.platform)};
    final names = [
      for (final target in targets)
        if (target.startsWith('skill/')) target.substring('skill/'.length),
    ]..sort();
    return [
      for (final name in names)
        if (metas[name] case final meta?)
          {'name': name, 'when_to_use': meta.whenToUse ?? meta.description, 'review': meta.review},
    ];
  }

  /// Packs chosen by `profile.stack` that exist for the profile's platform.
  List<Pack> selectedPacks(Profile profile) {
    final packs = catalog.packsFor(profile.platform);
    return [
      for (final MapEntry(key: family, value: choice) in profile.stack.entries) ?packs[family]?[choice.option],
    ];
  }

  /// Template context: the profile, `platform.*`, and `pack.<family>` params merged over pack defaults.
  Map<String, Object?> contextFor(Profile profile) {
    final context = profile.toContext();
    context['platform'] = catalog.platformFor(profile).toContext();
    context['pack'] = {
      for (final pack in selectedPacks(profile)) pack.family: {...pack.params, ...profile.stack[pack.family]!.params},
    };
    return context;
  }

  /// True when every `when` key matches; a list value means any of.
  static bool whenMatches(Map<String, Object?> when, Map<String, Object?> context) {
    for (final MapEntry(key: path, value: expected) in when.entries) {
      final actual = lookupPath(context, path);
      final ok = expected is List ? expected.contains(actual) : expected == actual;
      if (!ok) return false;
    }
    return true;
  }
}
