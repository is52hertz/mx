# notice

## Installation and releases

- `install.sh` installs SHA256-verified GitHub Release binaries into `~/.local/bin` for macOS/Linux AMD64/ARM64. It checks FFmpeg and optional ncm2mp3, asks via `/dev/tty` before installing missing dependencies, and only suggests commands without a terminal. It does not bootstrap Homebrew or Rust/Cargo.
- `scripts/build-release.sh` creates four archives and `SHA256SUMS` in ignored `dist/`. The tag-triggered release workflow runs verification and publishes these assets. Installer URLs target `is52hertz/mx`.
- Required FFmpeg still missing after installation yields exit 1 even though mx itself is installed; optional ncm2mp3 does not affect the exit code. Download remains a placeholder, so yt-dlp is not an installation requirement yet.

## Planned: `mx dl` (designed 2026-10-14, not started)

Agreed with the user; build on a `feat/dl` branch when picked up.

- Same "one row per setting" form as Convert (↑↓ row, ←→ value, enter start), reusing the progress bar.
- Batch: `mx dl url1 url2 …`, one shared set of options. A playlist URL downloads only that one video (`--no-playlist`).
- Probe each URL with `yt-dlp -J --no-playlist` (2–5 s, show "resolving…"): title, uploader, duration, real available heights + size estimate.
- Rows:
  - Type: Video / Audio only
  - Quality: Best + the heights that actually exist, e.g. `bv*[height<=1080]+ba/b`
  - Format: mp4 / mkv / webm (video) or mp3 / m4a / opus / original (audio)
  - Cookies: None / Chrome / Safari / Firefox (`--cookies-from-browser`)
  - Subtitles: None / Chinese / English / All, embedded in the video
  - Save to: current directory (default) ⇄ ~/Downloads
- Always `--embed-thumbnail --embed-metadata`.
- After it finishes, one key hands the downloaded files to the Convert panel.
- Progress comes from `--newline --progress-template`. Show the phase (video / audio / merge). On esc, clean up `.part` files.
- Output template `%(title)s [%(id)s].%(ext)s`. yt-dlp does not overwrite by default; still invoke it with an argv slice, never through a shell.
