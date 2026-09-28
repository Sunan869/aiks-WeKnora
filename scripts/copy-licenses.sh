#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
    echo "Usage: $0 DESTINATION_DIRECTORY" >&2
    exit 1
fi

license_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
license_dest="$1"

bash "$license_root/scripts/check-license-bundle.sh"

source_stage="$(mktemp -d)"
trap 'rm -rf "$source_stage"' EXIT

module_cache="$(cd "$license_root" && go env GOMODCACHE)"

while IFS=$'\t' read -r module_name module_version _checksum || [ -n "${module_name:-}" ]; do
    module_name="$(printf '%s' "${module_name:-}" | tr -d '\r')"
    module_version="$(printf '%s' "${module_version:-}" | tr -d '\r')"

    [ -n "$module_name" ] || continue

    (
        cd "$license_root"
        go mod download "$module_name@$module_version"
    )

    source_zip="$module_cache/cache/download/$module_name/@v/$module_version.zip"
    archive="${module_name##*/}-${module_version}.zip"

    test -s "$source_zip" || {
        echo "Missing Go module cache archive: $source_zip" >&2
        exit 1
    }

    cp "$source_zip" "$source_stage/$archive"
done < "$license_root/licenses/sources/modules.tsv"

(
    cd "$license_root"
    go mod verify
)

bash "$license_root/scripts/check-license-bundle.sh" "$source_stage"

mkdir -p "$license_dest/licenses"
cp "$license_root/LICENSE" "$license_root/THIRD_PARTY_NOTICES.md" "$license_dest/"
cp -R "$license_root/licenses/." "$license_dest/licenses/"
cp "$source_stage/"*.zip "$license_dest/licenses/sources/"
