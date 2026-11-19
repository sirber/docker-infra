#!/usr/bin/env bash
set -eu

release_url="https://github.com/sirber/docker-infra/releases/latest/download/infra"
install_dir="$HOME/.local/bin"
path_line='export PATH="$HOME/.local/bin:$PATH"'

if [ "$(uname -s)" != Linux ] || [ "$(uname -m)" != x86_64 ]; then
    echo "Error: infra releases currently support Linux x86_64 only." >&2
    exit 1
fi

mkdir -p "$install_dir"
tmp_file=$(mktemp "$install_dir/.infra.XXXXXX")
trap 'rm -f "$tmp_file"' EXIT

if command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp_file" "$release_url" || {
        echo "Error: failed to download infra." >&2
        exit 1
    }
elif command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$tmp_file" "$release_url" || {
        echo "Error: failed to download infra." >&2
        exit 1
    }
else
    echo "Error: install wget or curl to download infra." >&2
    exit 1
fi

if [ ! -s "$tmp_file" ]; then
    echo "Error: downloaded infra binary is empty." >&2
    exit 1
fi
chmod 755 "$tmp_file"
mv -f "$tmp_file" "$install_dir/infra"
echo "Installed infra to $install_dir/infra."

case ":${PATH-}:" in
    *":$install_dir:"*) exit 0 ;;
esac

echo "$install_dir is not on your PATH."
case "${SHELL-}" in
    bash|*/bash) shell_config="$HOME/.bashrc" ;;
    zsh|*/zsh) shell_config="$HOME/.zshrc" ;;
    sh|*/sh) shell_config="$HOME/.profile" ;;
    *) shell_config="" ;;
esac

if [ -n "$shell_config" ] && [ -f "$shell_config" ] &&
    grep -Fxq "$path_line" "$shell_config"; then
    echo "The PATH entry is already in $shell_config. Open a new shell or run: $path_line"
    exit 0
fi

if [ -n "$shell_config" ] && ( : </dev/tty ) 2>/dev/null; then
    printf 'Add it to %s? [y/N] ' "$shell_config" >&2
    if read -r answer </dev/tty; then
        case "$answer" in
            y|Y|yes|YES|Yes)
                printf '\n%s\n' "$path_line" >> "$shell_config"
                echo "Updated $shell_config. Open a new shell or run: $path_line"
                exit 0
                ;;
        esac
    fi
fi

echo "To use infra now, run: $path_line"
echo "Add that line to your shell configuration to make it permanent."
