#!/bin/sh
set -eu

repository="${STOAT_REPOSITORY:-wuuJiawei/stoat}"
version="${STOAT_VERSION:-}"
install_dir="${STOAT_INSTALL_DIR:-${HOME}/.local/bin}"
metadata_base="${STOAT_METADATA_BASE:-}"
download_base="${STOAT_DOWNLOAD_BASE:-}"
checksum_base="${STOAT_CHECKSUM_BASE:-}"
github_proxy="${STOAT_GITHUB_PROXY:-}"

say() { printf '%s\n' "$*"; }
fail() { printf 'stoat installer: %s\n' "$*" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: install.sh [options]
  --version VERSION          install an exact version, for example v1.0.0
  --install-dir DIRECTORY    default: ~/.local/bin
  --metadata-base URL        base containing latest.txt
  --download-base URL        base containing VERSION/archive
  --checksum-base URL        trusted base containing VERSION/checksums.txt
  --github-proxy URL         HTTPS prefix for GitHub archive downloads
  -h, --help                 show help
EOF
}

require_https_url() {
    case "$1" in
        https://*) ;;
        *) fail "$2 must use https://" ;;
    esac
    case "$1" in
        *[[:space:]]*) fail "$2 must not contain whitespace" ;;
    esac
}

require_version() {
    if ! printf '%s\n' "$1" | awk '/^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$/ { found=1 } END { exit !found }'; then
        fail "invalid version '$1'; expected vMAJOR.MINOR.PATCH"
    fi
}

platform_os() { /usr/bin/uname -s; }
platform_arch() { /usr/bin/uname -m; }

download() {
    source_url="$1"
    destination="$2"
    require_https_url "$source_url" "download URL"
    /usr/bin/curl --fail --silent --show-error --location \
        --proto '=https' --tlsv1.2 --retry 3 --retry-connrefused \
        --output "$destination" "$source_url"
}

resolve_latest() {
    destination="$1"
    if [ -n "$metadata_base" ]; then
        require_https_url "$metadata_base" "metadata base"
        download "${metadata_base%/}/latest.txt" "$destination"
        tr -d '[:space:]' < "$destination"
        return
    fi

    api_url="https://api.github.com/repos/${repository}/releases/latest"
    download "$api_url" "$destination"
    tag=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$destination" | head -n 1)
    [ -n "$tag" ] || fail "could not resolve the latest GitHub release"
    printf '%s\n' "$tag"
}

archive_url() {
    asset_name="$1"
    if [ -n "$download_base" ]; then
        printf '%s/%s/%s\n' "${download_base%/}" "$version" "$asset_name"
        return
    fi
    canonical="https://github.com/${repository}/releases/download/${version}/${asset_name}"
    if [ -n "$github_proxy" ]; then
        require_https_url "$github_proxy" "GitHub proxy"
        printf '%s%s\n' "${github_proxy%/}/" "$canonical"
        return
    fi
    printf '%s\n' "$canonical"
}

checksums_url() {
    if [ -n "$checksum_base" ]; then
        printf '%s/%s/checksums.txt\n' "${checksum_base%/}" "$version"
    elif [ -n "$metadata_base" ]; then
        printf '%s/%s/checksums.txt\n' "${metadata_base%/}" "$version"
    elif [ -n "$download_base" ]; then
        printf '%s/%s/checksums.txt\n' "${download_base%/}" "$version"
    else
        printf 'https://github.com/%s/releases/download/%s/checksums.txt\n' "$repository" "$version"
    fi
}

verify_archive() {
    archive="$1"
    checksums="$2"
    asset_name="$3"
    expected=$(awk -v name="$asset_name" '$2 == name || $2 == "*" name { print $1 }' "$checksums")
    [ -n "$expected" ] || fail "checksums.txt does not contain $asset_name"
    case "$expected" in
        *[!0-9a-fA-F]*|'') fail "invalid SHA-256 for $asset_name" ;;
    esac
    [ "${#expected}" -eq 64 ] || fail "invalid SHA-256 length for $asset_name"
    actual=$(/usr/bin/shasum -a 256 "$archive" | awk '{ print $1 }')
    [ "$actual" = "$expected" ] || fail "SHA-256 verification failed for $asset_name"

    entries=$(/usr/bin/tar -tzf "$archive") || fail "could not inspect release archive"
    [ -n "$entries" ] || fail "release archive is empty"
    previous_ifs=$IFS
    IFS='
'
    # Split the newline-delimited tar listing only.
    # shellcheck disable=SC2086
    for entry in $entries; do
        case "$entry" in
            stoat|README.md|LICENSE) ;;
            *) fail "release archive contains unexpected path: $entry" ;;
        esac
    done
    IFS=$previous_ifs
}

install_binary() {
    binary="$1"
    if [ ! -f "$binary" ] || [ -L "$binary" ]; then
        fail "release does not contain a regular stoat binary"
    fi
    [ ! -L "$install_dir" ] || fail "install directory must not be a symbolic link"
    /bin/mkdir -p "$install_dir"
    if [ ! -d "$install_dir" ] || [ ! -w "$install_dir" ]; then
        fail "install directory is not writable: $install_dir"
    fi
    target="$install_dir/stoat"
    [ ! -L "$target" ] || fail "refusing to replace symbolic link: $target"
    if [ -e "$target" ] && [ ! -f "$target" ]; then
        fail "refusing to replace non-regular path: $target"
    fi
    staged="$install_dir/.stoat.new.$$"
    trap 'rm -f "${staged:-}"; rm -rf "$temporary"' EXIT HUP INT TERM
    /usr/bin/install -m 0755 "$binary" "$staged"
    /bin/mv -f "$staged" "$target"
    installed="$target"
}

main() {
    while [ "$#" -gt 0 ]; do
        case "$1" in
            --version|--install-dir|--metadata-base|--download-base|--checksum-base|--github-proxy)
                [ "$#" -ge 2 ] || fail "$1 requires a value"
                case "$1" in
                    --version) version="$2" ;;
                    --install-dir) install_dir="$2" ;;
                    --metadata-base) metadata_base="$2" ;;
                    --download-base) download_base="$2" ;;
                    --checksum-base) checksum_base="$2" ;;
                    --github-proxy) github_proxy="$2" ;;
                esac
                shift 2
                ;;
            -h|--help) usage; return 0 ;;
            *) fail "unknown option: $1" ;;
        esac
    done

    [ "$(platform_os)" = "Darwin" ] || fail "requires macOS"
    case "$(platform_arch)" in
        arm64) architecture="arm64" ;;
        x86_64) architecture="amd64" ;;
        *) fail "unsupported architecture: $(platform_arch)" ;;
    esac
    require_https_url "https://github.com/${repository}" "repository"
    if [ -z "$install_dir" ] || [ "${install_dir#/}" = "$install_dir" ]; then
        fail "install directory must be absolute"
    fi

    temporary=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/stoat-install.XXXXXX")
    trap 'rm -rf "$temporary"' EXIT HUP INT TERM
    if [ -z "$version" ]; then
        version=$(resolve_latest "$temporary/latest")
    fi
    require_version "$version"

    asset="stoat-${version}-darwin-${architecture}.tar.gz"
    archive="$temporary/$asset"
    checksums="$temporary/checksums.txt"
    say "Downloading Stoat $version for $architecture..."
    download "$(archive_url "$asset")" "$archive"
    download "$(checksums_url)" "$checksums"
    verify_archive "$archive" "$checksums" "$asset"
    /usr/bin/tar -xzf "$archive" -C "$temporary" stoat
    if [ ! -f "$temporary/stoat" ] || [ -L "$temporary/stoat" ]; then
        fail "release does not contain a regular stoat binary"
    fi
    actual_version=$("$temporary/stoat" version 2>/dev/null || true)
    [ "$actual_version" = "$version" ] || fail "installed binary reported '$actual_version', expected '$version'"
    install_binary "$temporary/stoat"
    say "Installed Stoat $version to $installed"
    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *) say "Add $install_dir to PATH before running stoat." ;;
    esac
}

if [ "${STOAT_INSTALLER_LIB_ONLY:-0}" != "1" ]; then
    main "$@"
fi
