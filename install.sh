#!/usr/bin/env bash
set -euo pipefail

repo="is52hertz/mx"
fail() { printf '错误：%s\n' "$*" >&2; exit 1; }

# Read from the terminal even when this script arrives through curl | bash.
ask() {
    local answer
    if ! { exec 3<>/dev/tty; } 2>/dev/null; then
        return 1
    fi
    printf '%s [y/N] ' "$1" >&3
    if ! IFS= read -r answer <&3; then answer=""; fi
    exec 3>&-
    case "$answer" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail "目前仅支持 macOS/Linux。" ;;
esac
case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "目前仅支持 AMD64/ARM64。" ;;
esac
for tool in curl tar awk mktemp; do
    command -v "$tool" >/dev/null 2>&1 || fail "缺少安装工具：$tool"
done
if command -v sha256sum >/dev/null 2>&1; then
    hash=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
    hash=(shasum -a 256)
else
    fail "需要 sha256sum 或 shasum 来校验下载文件。"
fi

tmp=$(mktemp -d)
staged=""
trap 'rm -rf "$tmp"; if [[ -n "$staged" ]]; then rm -f "$staged"; fi' EXIT
release_url=$(curl --proto '=https' --proto-redir '=https' -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
version=${release_url##*/}
[[ "$version" == v* && "$version" != *[^a-zA-Z0-9._-]* ]] || fail "无法确定最新版本。"
asset="mx_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"
printf '下载 mx %s (%s/%s)…\n' "$version" "$os" "$arch"
curl --proto '=https' --proto-redir '=https' -fsSL "$base/$asset" -o "$tmp/$asset"
curl --proto '=https' --proto-redir '=https' -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"
expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$tmp/SHA256SUMS")
actual=$("${hash[@]}" "$tmp/$asset")
actual=${actual%% *}
[[ -n "$expected" && "$actual" == "$expected" ]] || fail "SHA256 校验失败；未安装下载的文件。"
tar -xzf "$tmp/$asset" -C "$tmp" mx
[[ -f "$tmp/mx" && ! -L "$tmp/mx" ]] || fail "安装包中没有有效的 mx。"
mkdir -p "$HOME/.local/bin"
staged=$(mktemp "$HOME/.local/bin/.mx-install-XXXXXX")
cp "$tmp/mx" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$HOME/.local/bin/mx"
staged=""
printf '已安装到 ~/.local/bin/mx。\n'
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) printf '请在 shell 配置中加入，再打开终端：\n  export PATH="$HOME/.local/bin:$PATH"\n' ;;
esac

missing=0
for tool in ffmpeg ffprobe; do
    if command -v "$tool" >/dev/null 2>&1; then
        printf '✓ %s\n' "$tool"
    else
        printf '缺少 %s（媒体信息/转换需要 FFmpeg）。\n' "$tool"
        missing=1
    fi
done
if [[ "$missing" == 1 ]]; then
    package=()
    if command -v brew >/dev/null 2>&1; then
        package=(brew install ffmpeg)
    elif [[ "$os" == linux ]]; then
        if command -v apt-get >/dev/null 2>&1; then
            package=(apt-get install -y ffmpeg)
        elif command -v dnf >/dev/null 2>&1; then
            package=(dnf install -y ffmpeg)
        elif command -v pacman >/dev/null 2>&1; then
            package=(pacman -S --needed --noconfirm ffmpeg)
        fi
        if [[ ${#package[@]} -gt 0 && "$EUID" != 0 ]]; then
            if command -v sudo >/dev/null 2>&1; then
                package=(sudo "${package[@]}")
            else
                printf '缺少 sudo；请让管理员安装 FFmpeg。\n'
                package=()
            fi
        fi
    fi
    if [[ ${#package[@]} -gt 0 ]]; then
        printf '安装命令：'; printf ' %s' "${package[@]}"; printf '\n'
        if ask '是否帮你安装 FFmpeg？（Linux 可能需要管理员权限）'; then
            if ! "${package[@]}"; then
                printf 'FFmpeg 安装失败；请处理包管理器错误后重新运行安装脚本。\n' >&2
            fi
        fi
    else
        printf '请通过系统包管理器安装 FFmpeg：https://ffmpeg.org/download.html\n'
        if [[ "$os" == darwin ]]; then
            printf 'macOS 可先安装 Homebrew（https://brew.sh），再运行：brew install ffmpeg\n'
        fi
    fi
fi

if command -v ncm2mp3 >/dev/null 2>&1; then
    printf '✓ ncm2mp3（可选，处理 .ncm 文件）\n'
else
    printf '缺少可选依赖 ncm2mp3；仅 .ncm 文件需要它。\n'
    if command -v cargo >/dev/null 2>&1 && cargo --version >/dev/null 2>&1; then
        printf '✓ Cargo；安装命令：cargo install ncm2mp3 --locked\n'
        if ask '是否帮你安装 ncm2mp3？（需要 Rust 编译环境，可能耗时几分钟）'; then
            if cargo install ncm2mp3 --locked; then
                case ":$PATH:" in
                    *":${CARGO_HOME:-$HOME/.cargo}/bin:"*) ;;
                    *) printf '请将 Cargo 的 bin 目录加入 PATH：\n  export PATH="${CARGO_HOME:-$HOME/.cargo}/bin:$PATH"\n' ;;
                esac
            else
                printf 'ncm2mp3 安装失败；普通媒体功能仍可使用。\n' >&2
            fi
        fi
    else
        printf '未检测到可用的 Cargo。请先安装或修复 Rust/Cargo：https://rustup.rs\n确保 ~/.cargo/bin 在 PATH 中，随后运行：cargo install ncm2mp3 --locked\n'
    fi
fi
printf 'Download 尚未实现，当前不需要安装 yt-dlp。\n'
if ! command -v ffmpeg >/dev/null 2>&1 || ! command -v ffprobe >/dev/null 2>&1; then
    printf 'mx 已安装，但 FFmpeg 依赖仍缺失；补齐后即可使用媒体信息和转换。\n' >&2
    exit 1
fi
printf '安装完成。运行 mx 开始使用。\n'
