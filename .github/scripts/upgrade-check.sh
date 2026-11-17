#!/bin/sh
# Upgrade check: the previous release syncs a project with every agent, then the build under test re-syncs it. The
# re-sync must succeed without --force (a required sync is fine, a conflict is not) and doctor must report no fail.
# Usage: upgrade-check.sh <previous dashcrew> <dashcrew under test> <project.yaml>
set -eu
old=$1
new=$2
profile=$3

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
project=$work/project
mkdir -p "$project/.ai" "$work/home" "$work/bin"
cp "$profile" "$project/.ai/project.yaml"
ln -s "$new" "$work/bin/dashcrew"
export HOME="$work/home"
git -C "$project" init -q
git -C "$project" config user.email ci@example.invalid
git -C "$project" config user.name ci

echo "== $("$old" --version): sync"
(cd "$project" && "$old" sync)
git -C "$project" add -A
git -C "$project" commit -qm "synced by the previous release"

echo "== $("$new" --version): sync"
(cd "$project" && "$new" sync)

echo "== doctor"
report=$(cd "$project" && PATH="$work/bin:$PATH" "$new" doctor) || true
echo "$report"
if echo "$report" | grep -q '^fail'; then
  echo "::error::doctor reports a failing check after the upgrade"
  exit 1
fi
