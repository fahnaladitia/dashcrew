import '../catalog/catalog.dart';
import '../profile/profile.dart';
import 'guard.dart';

/// Builds the guard policy that `sync` writes to `.ai/guard.json`.
/// Push and history-rewriting git commands stay blocked even when `policy.agent_commit` is true.
GuardPolicy compileGuardPolicy(Profile profile, Catalog catalog) {
  final forbiddenImports = <String>[];
  final analytics = profile.stack['analytics'];
  if (analytics != null) {
    final defaults = catalog.packs['analytics']?[analytics.option]?.params ?? const {};
    final vendorImport = {...defaults, ...analytics.params}['vendor_import'];
    if (vendorImport is String && vendorImport.isNotEmpty) forbiddenImports.add(vendorImport);
  }

  return GuardPolicy(
    blockedGit: {...GuardPolicy.alwaysBlockedGit, if (profile.policy['agent_commit'] != true) 'commit'},
    blockFormatters: profile.policy['formatter'] != true,
    plansDir: profile.paths['plans'] as String,
    roleCapability: {for (final role in catalog.roles.values) role.name: role.capability},
    forbiddenImports: forbiddenImports,
    localPackages: [for (final package in profile.project['local_packages'] as List) package.toString()],
    planGate: profile.policy['plan_gate'] == true,
  );
}
