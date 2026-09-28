#!/usr/bin/env bash
set -euo pipefail

# Assemble notices and verified corresponding source for backend/desktop releases.
if [ "$#" -ne 1 ] || [ -z "$1" ]; then
    echo "Usage: $0 DESTINATION_DIRECTORY" >&2
    exit 1
fi

license_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
license_dest="$1"
bash "${license_root}/scripts/check-license-bundle.sh"

# Use Go's module cache and configured GOPROXY, including offline cache hits.
# Archives are release inputs, not files vendored in the source repository.
source_stage="$(mktemp -d)"
trap 'rm -rf "${source_stage}"' EXIT
module_cache="$(cd "${license_root}" && go env GOMODCACHE)"
while read -r module_name module_version _checksum; do
    (cd "${license_root}" && go mod download "${module_name}@${module_version}")
    cp "${module_cache}/cache/download/${module_name}/@v/${module_version}.zip" \
        "${source_stage}/${module_name##*/}-${module_version}.zip"
done < "${license_root}/licenses/sources/modules.tsv"

# Go verifies downloaded module contents against go.sum using its canonical
# module hash. Raw module ZIP bytes can differ between GOPROXY implementations
# even when the canonical module content is identical, so do not use the ZIP
# file's SHA-256 as a cross-proxy integrity check.
(cd "${license_root}" && go mod verify)
bash "${license_root}/scripts/check-license-bundle.sh" "${source_stage}"

mkdir -p "${license_dest}/licenses"
cp "${license_root}/LICENSE" "${license_root}/THIRD_PARTY_NOTICES.md" "${license_dest}/"
cp -R "${license_root}/licenses/." "${license_dest}/licenses/"
cp "${source_stage}/"*.zip "${license_dest}/licenses/sources/"
