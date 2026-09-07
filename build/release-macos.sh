#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "${version}" ]]; then
  echo "version is required" >&2
  exit 1
fi
if [[ ! "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be semantic: ${version}" >&2
  exit 1
fi
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "macOS release packaging must run on macOS" >&2
  exit 1
fi
if [[ "$(uname -m)" != "arm64" ]]; then
  echo "macOS release packaging must run on Apple Silicon" >&2
  exit 1
fi
if ! command -v wails3 >/dev/null 2>&1; then
  echo "wails3 v3.0.0-beta.16 is required" >&2
  exit 1
fi
if [[ "$(wails3 version 2>&1)" != *"v3.0.0-beta.16"* ]]; then
  echo "wails3 v3.0.0-beta.16 is required" >&2
  exit 1
fi

config_backup="$(mktemp)"
plist_backup="$(mktemp)"
cp build/config.yml "${config_backup}"
cp build/darwin/Info.plist "${plist_backup}"
cleanup() {
  cp "${config_backup}" build/config.yml
  cp "${plist_backup}" build/darwin/Info.plist
  rm -f "${config_backup}" "${plist_backup}"
}
trap cleanup EXIT

node - "${version}" <<'NODE'
const fs = require('node:fs');
const version = process.argv[2];
const path = 'build/config.yml';
const config = fs.readFileSync(path, 'utf8');
const updated = config.replace(/(info:\n(?:.*\n)*?  version:) "[^"]+"/, `$1 "${version}"`);
if (updated === config) throw new Error('unable to set build/config.yml info.version');
fs.writeFileSync(path, updated);
NODE

/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${version}" build/darwin/Info.plist
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${version}" build/darwin/Info.plist
wails3 task darwin:package:dmg ARCH=arm64

app_path="bin/lumivue.app"
dmg_path="bin/lumivue.dmg"
if [[ ! -d "${app_path}" || ! -f "${dmg_path}" ]]; then
  echo "Wails v3 package output is incomplete" >&2
  exit 1
fi

bundle_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "${app_path}/Contents/Info.plist")"
short_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "${app_path}/Contents/Info.plist")"
if [[ "${bundle_version}" != "${version}" || "${short_version}" != "${version}" ]]; then
  echo "application bundle version does not match ${version}" >&2
  exit 1
fi

mkdir -p build/release
output="build/release/lumivue-${version}-darwin-arm64.dmg"
mv "${dmg_path}" "${output}"
printf '%s\n' "${output}"
