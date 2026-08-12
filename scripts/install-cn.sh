#!/bin/sh
set -eu

installer_url="${STOAT_INSTALLER_URL:-https://stoat.lighting.pub/install.sh}"
github_proxy="${STOAT_GITHUB_PROXY:-}"
download_base="${STOAT_DOWNLOAD_BASE:-}"

if [ -z "$download_base" ] && [ -z "$github_proxy" ]; then
    download_base="https://stoat.lighting.pub/releases"
fi

case "$installer_url" in
    https://*) ;;
    *) printf 'stoat installer: STOAT_INSTALLER_URL must use https://\n' >&2; exit 1 ;;
esac

temporary=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/stoat-cn-install.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
/usr/bin/curl --fail --silent --show-error --location \
    --proto '=https' --tlsv1.2 --retry 3 --retry-all-errors \
    --output "$temporary/install.sh" "$installer_url"

STOAT_METADATA_BASE="${STOAT_METADATA_BASE:-https://stoat.lighting.pub/releases}" \
STOAT_CHECKSUM_BASE="${STOAT_CHECKSUM_BASE:-https://stoat.lighting.pub/releases}" \
STOAT_DOWNLOAD_BASE="$download_base" \
STOAT_GITHUB_PROXY="$github_proxy" \
    /bin/sh "$temporary/install.sh" "$@"
