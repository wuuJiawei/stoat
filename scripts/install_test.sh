#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/stoat-installer-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

export STOAT_INSTALLER_LIB_ONLY=1
# shellcheck source=scripts/install.sh
source "$repo_root/scripts/install.sh"

platform_os() { printf 'Darwin\n'; }
platform_arch() { printf 'arm64\n'; }
require_https_url() { :; }
download() {
    cp "${1#file://}" "$2"
}

release_root="$test_root/releases/v1.0.0"
mkdir -p "$release_root/stage" "$test_root/home" "$test_root/bin"
cat > "$release_root/stage/stoat" <<'EOF'
#!/bin/sh
[ "${1:-}" = version ] && printf 'v1.0.0\n'
EOF
chmod 0755 "$release_root/stage/stoat"
printf 'readme\n' > "$release_root/stage/README.md"
printf 'license\n' > "$release_root/stage/LICENSE"
tar -C "$release_root/stage" -czf "$release_root/stoat-v1.0.0-darwin-arm64.tar.gz" stoat README.md LICENSE
(cd "$release_root" && shasum -a 256 stoat-v1.0.0-darwin-arm64.tar.gz > checksums.txt)
printf 'v1.0.0\n' > "$test_root/releases/latest.txt"

distribution_root="$test_root/releases"
resolve_latest() { printf 'v1.0.0\n'; }
archive_url() { printf 'file://%s/%s/%s\n' "$distribution_root" "$version" "$1"; }
checksums_url() { printf 'file://%s/%s/checksums.txt\n' "$distribution_root" "$version"; }

help_output="$(main --help)"
case "$help_output" in
    *metadata-base*|*download-base*|*checksum-base*|*github-proxy*)
        echo "removed distribution option remains in installer help" >&2
        exit 1
        ;;
esac

export version=""
main \
    --install-dir "$test_root/bin"

test -x "$test_root/bin/stoat"
test "$("$test_root/bin/stoat" version)" = "v1.0.0"

bad_root="$test_root/bad/v1.0.0"
mkdir -p "$bad_root/stage"
cp "$release_root/stage/stoat" "$bad_root/stage/stoat"
printf 'bad\n' > "$bad_root/stage/../escape"
tar -C "$bad_root/stage" -czf "$bad_root/stoat-v1.0.0-darwin-arm64.tar.gz" stoat ../escape
(cd "$bad_root" && shasum -a 256 stoat-v1.0.0-darwin-arm64.tar.gz > checksums.txt)

export version=""
distribution_root="$test_root/bad"
if (main --version v1.0.0 --install-dir "$test_root/bin-2") >/dev/null 2>&1; then
    echo "unsafe archive was accepted" >&2
    exit 1
fi

echo "installer integration tests passed"
