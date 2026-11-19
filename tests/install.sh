#!/usr/bin/env bash
set -eu

repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
mkdir -p "$test_dir/bin" "$test_dir/home"

cat > "$test_dir/bin/wget" <<'EOF'
#!/usr/bin/env bash
if [ "${DOWNLOAD_FAIL-}" = 1 ]; then
    printf 'partial' > "$2"
    exit 1
fi
printf '#!/bin/sh\necho installed\n' > "$2"
EOF
cat > "$test_dir/bin/uname" <<'EOF'
#!/usr/bin/env bash
case "$1" in
    -s) echo "${TEST_OS-Linux}" ;;
    -m) echo "${TEST_ARCH-x86_64}" ;;
esac
EOF
chmod +x "$test_dir/bin/wget" "$test_dir/bin/uname"
export HOME="$test_dir/home" SHELL=/bin/bash
export PATH="$test_dir/bin:/usr/bin:/bin"

# Non-interactive installation must give actionable PATH instructions.
setsid bash "$repo_dir/install.sh" > "$test_dir/output" 2>&1
test -x "$HOME/.local/bin/infra"
grep -Fq 'Add that line to your shell configuration' "$test_dir/output"
test ! -e "$HOME/.bashrc"

# A failed download must not replace an existing installation.
if DOWNLOAD_FAIL=1 setsid bash "$repo_dir/install.sh" > "$test_dir/output" 2>&1; then
    echo "Failed download unexpectedly succeeded" >&2
    exit 1
fi
test "$("$HOME/.local/bin/infra")" = installed
grep -Fq 'failed to download' "$test_dir/output"
test "$(find "$HOME/.local/bin" -name '.infra.*' | wc -l)" -eq 0

if TEST_ARCH=aarch64 setsid bash "$repo_dir/install.sh" > "$test_dir/output" 2>&1; then
    echo "Unsupported architecture unexpectedly succeeded" >&2
    exit 1
fi
grep -Fq 'Linux x86_64 only' "$test_dir/output"

# The installer must read the answer from the terminal even when piped to bash.
printf 'y\n' | script -q -e -c "cat '$repo_dir/install.sh' | bash" /dev/null > "$test_dir/output"
path_line='export PATH="$HOME/.local/bin:$PATH"'
grep -Fxq "$path_line" "$HOME/.bashrc"
printf 'y\n' | script -q -e -c "cat '$repo_dir/install.sh' | bash" /dev/null > "$test_dir/output"
test "$(grep -Fxc "$path_line" "$HOME/.bashrc")" -eq 1

PATH="$HOME/.local/bin:$PATH" setsid bash "$repo_dir/install.sh" > "$test_dir/output" 2>&1
if grep -Fq 'not on your PATH' "$test_dir/output"; then
    echo "Installer did not recognize the existing PATH entry" >&2
    exit 1
fi
echo "Installer tests passed."
