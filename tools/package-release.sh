#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 4 ]]; then
  echo 'usage: package-release.sh version arch binary output-directory' >&2
  exit 1
fi
version=$1
arch=$2
binary=$3
output=$4
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$ ]] || { echo 'invalid release version' >&2; exit 1; }
[[ "$arch" == amd64 || "$arch" == arm64 ]] || { echo 'unsupported release architecture' >&2; exit 1; }
[[ -f "$binary" && -x "$binary" ]] || { echo 'release binary must be an executable file' >&2; exit 1; }
[[ -n "$output" ]] || { echo 'release output directory is required' >&2; exit 1; }
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
name="wraith_${version}_linux_${arch}"

mkdir -p -- "$output"
staging=$(mktemp -d)
archive=''
trap 'rm -rf -- "$staging"; if [[ -n "$archive" ]]; then rm -f -- "$archive"; fi' EXIT
package="$staging/$name"
mkdir -p "$package/licenses" "$package/examples" "$package/third_party/gopher-lua"
cp -- "$binary" "$package/wraith"
cp -- "$root/README.md" "$root/LICENSE" "$root/NOTICE" "$package/"
cp -R -- "$root/docs" "$package/"
cp -R -- "$root/examples/plugins" "$package/examples/"
cp -- "$root/third_party/README.md" "$package/third_party/"
cp -- "$root/third_party/gopher-lua/LICENSE" "$package/third_party/gopher-lua/"
cp -- "$root/third_party/gopher-lua/LICENSE" "$package/licenses/gopher-lua.txt"
mkdir -p "$package/internal/distribution/assets/seeds"
cp -- "$root/internal/distribution/assets/manifest.json" "$package/internal/distribution/assets/"
cp -- "$root/internal/distribution/assets/seeds/"*.bundle "$package/internal/distribution/assets/seeds/"

archive=$(mktemp "$output/.wraith-release-XXXXXX")
tar -C "$staging" -czf "$archive" "$name"
chmod 644 "$archive"
mv -f -- "$archive" "$output/$name.tar.gz"
