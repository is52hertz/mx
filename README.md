# mx

一个本地终端媒体工具：浏览文件、查看媒体信息、批量转换格式。输入 `mx` 打开 TUI，选择文件后直接查看或转换。

基于 Go、Bubble Tea v2 和 Lip Gloss v2，调用本机 FFmpeg 和可选的 ncm2mp3。支持英文/简体中文界面，按 `L` 切换。

## 一键安装

支持 **macOS / Linux，AMD64 / ARM64**，无需安装 Go：

```bash
curl -fsSL https://raw.githubusercontent.com/is52hertz/mx/main/install.sh | bash
```

安装脚本从 [GitHub Releases](https://github.com/is52hertz/mx/releases/latest) 下载对应的预编译包，校验 SHA256，并安装到 `~/.local/bin/mx`。再次运行同一命令即可升级。

如果希望先审阅脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/is52hertz/mx/main/install.sh -o install-mx.sh
less install-mx.sh
bash install-mx.sh
```

若终端找不到 `mx`，在 `~/.zshrc` 或 `~/.bashrc` 加入并重新打开终端：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### 依赖检查与安装

脚本检测依赖，缺失时展示安装命令，并在有交互终端时询问是否代为安装，默认 **不安装**。即使使用 `curl | bash`，也能通过终端回答。没有交互终端时仅提示。

| 工具 | 用途 | 安装方式 |
| --- | --- | --- |
| `ffprobe` / `ffmpeg` | 媒体信息 / 转换 | macOS：`brew install ffmpeg`；Debian/Ubuntu：`sudo apt-get install ffmpeg` |
| `ncm2mp3`（可选） | 查看、解密 `.ncm` 文件 | `cargo install ncm2mp3 --locked` |

FFmpeg 自动安装支持已有的 Homebrew、apt-get、dnf、pacman。Linux 可能需要 sudo；包管理器必须能提供 FFmpeg，如包索引或软件源未配置，请先按发行版说明处理。安装脚本不会自行添加软件源。

安装 ncm2mp3 前检查 Cargo；缺少 Cargo 时提示先通过 [rustup](https://rustup.rs) 安装 Rust 工具链。脚本不会自行安装 Homebrew 或 Rust。Cargo 安装的软件通常在 `~/.cargo/bin`，该目录也需要在 `PATH` 中。

缺少必需的 FFmpeg 时，`mx` 二进制仍会安装，但脚本退出码为 `1` 并提示补齐依赖。缺少可选的 ncm2mp3 不影响普通媒体功能。安装脚本自身需要 Bash、curl、tar、awk、mktemp，以及 sha256sum 或 shasum。

## 使用

```bash
mx                              # 浏览当前目录
mx info movie.mp4 song.flac      # 查看一个或多个文件的信息
mx i movie.mp4                  # info 的简写
mx cv movie.mov mp4             # 转换，可预选目标格式
mx c song.flac track.wav mp3    # 批量转换，使用同一组设置
mx cv song.ncm original         # 仅解密，保留原始音频格式
mx --help
```

命令会打开对应 TUI 面板，转换需确认设置后按 Enter 开始。

| 面板 | 按键 |
| --- | --- |
| 全局 | `Tab` / `Shift+Tab` / `1`–`4` 切换面板；`L` 切换语言；`q` 退出 |
| Files | `↑↓` / `jk` 移动；空格选择；`a` 全选/取消当前目录媒体；Enter 打开目录；`←` 返回上级；`i` 查看；`c` 转换 |
| Info | `↑↓`、Page Up / Page Down、Home / End 滚动 |
| Convert | `↑↓` 选择设置；`←→` 改值；Enter 开始；Esc 取消当前批次 |

Files 的选择可跨目录保留。未选择文件时，`i` / `c` 使用光标所在文件。

## 功能与限制

- **Files**：浏览目录，后台读取时长、编码、分辨率和大小，窄终端自动减少列。
- **Info**：展示容器、视频、音频、字幕和元数据，支持多个文件。
- **Convert**：支持 mp4、mkv、webm、gif、mp3、m4a、flac、wav、opus；可选质量、分辨率、只保留开头/结尾时长。兼容时提供流复制，避免重编码；流复制裁剪受关键帧影响。
- **NCM**：可仅解密到原格式，也可解密后转换，并选择保留解密文件。
- 输出保存到输入文件同目录，文件名冲突时增加编号。取消/失败会清理本次未完成输出。
- mp3/m4a/flac 尽可能保留内嵌封面；wav/opus 不保留。
- **Download 尚未实现**：`mx dl` / `mx d` 目前只打开占位面板，无需安装 yt-dlp。
- 需要交互终端；当前不提供 Windows 安装包。可在 Windows 的 WSL Linux 环境使用 Linux 安装脚本。

## 从源码构建

需要 `go.mod` 指定的 Go 版本或更新版本，以及上述运行时依赖：

```bash
git clone https://github.com/is52hertz/mx.git
cd mx
go vet ./...
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/mx" .
```

## 开发与发布

```bash
bash -n install.sh scripts/build-release.sh
python3 -m unittest discover -s tests -v
go vet ./...
go build -o mx .
bash scripts/build-release.sh
```

`dist/` 中生成四个平台包和 `SHA256SUMS`。推送 `v*` 标签会触发 GitHub Actions 验证、构建并发布 Release。

卸载：删除 `~/.local/bin/mx`；外部工具由各自的包管理器管理。
