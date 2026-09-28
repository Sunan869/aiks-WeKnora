#!/usr/bin/env bash
set -euo pipefail

license_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="${1:-}"
if [ -n "${source_dir}" ]; then
    source_dir="$(cd "${source_dir}" && pwd)"
fi
cd "${license_root}"

# A retained build-graph dependency would bring the GPL converter back even if
# application code no longer imported it directly.
if grep -Eq 'github.com/(longbridgeapp/opencc|liuzl/(da|cedar-go)|adamzy/cedar-go)([[:space:]]|$)' go.mod go.sum; then
    echo "Removed conversion dependencies have reappeared in go.mod/go.sum" >&2
    exit 1
fi

test -s licenses/sources/modules.tsv
while read -r module_name module_version checksum; do
    # Git for Windows may check TSV files out with CRLF. Strip a trailing CR
    # from parsed fields so validation is platform-independent.
    module_name="${module_name%    actual_version="$(awk -v name="${module_name}" '$1 == name { print $2 }' go.mod)"
    if [ "${actual_version}" != "${module_version}" ]; then
        echo "Update the source manifest for ${module_name}: expected ${module_version}, got ${actual_version}" >&2
        exit 1
    fi
    if [ -n "${source_dir}" ]; then
        archive="${module_name##*/}-${module_version}.zip"
        test -s "${source_dir}/${archive}" || {
            echo "Missing source archive for ${module_name}@${module_version}" >&2
            exit 1
        }
        # Do not compare the raw ZIP SHA-256 here. Different GOPROXY servers can
        # return byte-different ZIP archives for the same canonical Go module.
        # copy-licenses.sh runs 'go mod verify', which checks module content
        # against go.sum using Go's stable module checksum algorithm.
    fi
done < licenses/sources/modules.tsv

for file in LICENSE THIRD_PARTY_NOTICES.md licenses/OpenCC-Apache-2.0.txt \
    licenses/go-sql-driver-mysql-MPL-2.0.txt licenses/go-m1cpu-MPL-2.0.txt \
    licenses/Wails-MIT.txt; do
    test -s "${file}"
done
\r'}"
    module_version="${module_version%    actual_version="$(awk -v name="${module_name}" '$1 == name { print $2 }' go.mod)"
    if [ "${actual_version}" != "${module_version}" ]; then
        echo "Update the source manifest for ${module_name}: expected ${module_version}, got ${actual_version}" >&2
        exit 1
    fi
    if [ -n "${source_dir}" ]; then
        archive="${module_name##*/}-${module_version}.zip"
        test -s "${source_dir}/${archive}" || {
            echo "Missing source archive for ${module_name}@${module_version}" >&2
            exit 1
        }
        # Do not compare the raw ZIP SHA-256 here. Different GOPROXY servers can
        # return byte-different ZIP archives for the same canonical Go module.
        # copy-licenses.sh runs 'go mod verify', which checks module content
        # against go.sum using Go's stable module checksum algorithm.
    fi
done < licenses/sources/modules.tsv

for file in LICENSE THIRD_PARTY_NOTICES.md licenses/OpenCC-Apache-2.0.txt \
    licenses/go-sql-driver-mysql-MPL-2.0.txt licenses/go-m1cpu-MPL-2.0.txt \
    licenses/Wails-MIT.txt; do
    test -s "${file}"
done
\r'}"
    checksum="${checksum%    actual_version="$(awk -v name="${module_name}" '$1 == name { print $2 }' go.mod)"
    if [ "${actual_version}" != "${module_version}" ]; then
        echo "Update the source manifest for ${module_name}: expected ${module_version}, got ${actual_version}" >&2
        exit 1
    fi
    if [ -n "${source_dir}" ]; then
        archive="${module_name##*/}-${module_version}.zip"
        test -s "${source_dir}/${archive}" || {
            echo "Missing source archive for ${module_name}@${module_version}" >&2
            exit 1
        }
        # Do not compare the raw ZIP SHA-256 here. Different GOPROXY servers can
        # return byte-different ZIP archives for the same canonical Go module.
        # copy-licenses.sh runs 'go mod verify', which checks module content
        # against go.sum using Go's stable module checksum algorithm.
    fi
done < licenses/sources/modules.tsv

for file in LICENSE THIRD_PARTY_NOTICES.md licenses/OpenCC-Apache-2.0.txt \
    licenses/go-sql-driver-mysql-MPL-2.0.txt licenses/go-m1cpu-MPL-2.0.txt \
    licenses/Wails-MIT.txt; do
    test -s "${file}"
done
\r'}"
    [[ "${checksum}" =~ ^[0-9a-f]{64}$ ]] || { echo "Invalid source checksum for ${module_name}" >&2; exit 1; }
    actual_version="$(awk -v name="${module_name}" '$1 == name { print $2 }' go.mod)"
    if [ "${actual_version}" != "${module_version}" ]; then
        echo "Update the source manifest for ${module_name}: expected ${module_version}, got ${actual_version}" >&2
        exit 1
    fi
    if [ -n "${source_dir}" ]; then
        archive="${module_name##*/}-${module_version}.zip"
        test -s "${source_dir}/${archive}" || {
            echo "Missing source archive for ${module_name}@${module_version}" >&2
            exit 1
        }
        # Do not compare the raw ZIP SHA-256 here. Different GOPROXY servers can
        # return byte-different ZIP archives for the same canonical Go module.
        # copy-licenses.sh runs 'go mod verify', which checks module content
        # against go.sum using Go's stable module checksum algorithm.
    fi
done < licenses/sources/modules.tsv

for file in LICENSE THIRD_PARTY_NOTICES.md licenses/OpenCC-Apache-2.0.txt \
    licenses/go-sql-driver-mysql-MPL-2.0.txt licenses/go-m1cpu-MPL-2.0.txt \
    licenses/Wails-MIT.txt; do
    test -s "${file}"
done
