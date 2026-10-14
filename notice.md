# notice

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
