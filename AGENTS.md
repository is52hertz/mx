<METADATA>
# AI Agent Instructions

This file is the single source of truth for all AI coding assistants working on this project.
Tool-specific files (`CLAUDE.md`) point here.

Keep it to durable, cross-session guidance — product, rules, standards, security, handoff. Session scratch (current branch, this round's progress, temporary state) does not belong here; put it in the End-of-Session Summary or `notice.md`.
</METADATA>

<PRODUCT>
## Product

`mx` is a single-user, local terminal tool (macOS first) that unifies media info, download and format conversion behind one short command that opens a TUI. It shells out to locally installed CLIs (`ffprobe`, `ffmpeg`, `yt-dlp`, `ncm2mp3` — the latter via `cargo install ncm2mp3`, since Homebrew 7 refuses its URL formula) rather than reimplementing them. Priority order for design tradeoffs: TUI intuitiveness > efficiency > everything else.
</PRODUCT>

<PROJECT_PHASE>
## Project Phase

- **mx** (Go, Bubble Tea v2 + Lip Gloss v2): subcommand routing (`mx`, `info|i`, `dl|d`, `cv|c`) and a three-tab TUI. **Info** works: `mx info <file>` runs ffprobe async (`info.go`) and renders General + per-stream sections; no scrolling yet (long stream lists overflow). **Convert** works: `mx cv <file>... [fmt]` — a form of rows (format / quality incl. ⚡stream copy / resolution / trim keep-first|keep-last N s), ↑↓ row, ←→ value; batch runs sequentially with an ffmpeg `-progress` bar; esc cancels, q quits (both kill ffmpeg and delete the partial output). `.ncm` inputs (`ncm.go`) are probed with `ncm2mp3 info` and offer an "Original format (mp3/flac)" target = decrypt only; other targets decrypt then convert, with an optional "keep decrypted" row. ncm2mp3 may exit 0 on per-file failure and skips (exit 0) on conflicts, so mx always decrypts into a fresh `.mx-ncm-*` dir next to the input and checks for its `✗` lines. Download is a placeholder. UI is bilingual (EN / 简体中文): initial language from `LC_ALL`/`LC_MESSAGES`/`LANG`, `L` toggles at runtime; not persisted. Verified by manual tmux smoke tests.
</PROJECT_PHASE>

<CORE_RULES>
## Development Rules

### Communication Language
- User-facing output — explanations, questions, status updates, summaries, anything the user reads — must be in Simplified Chinese (简体中文).
- Software and internal work stays in English: code, identifiers, comments, commit messages, logs, test names, and your own reasoning/analysis.
- This governs presentation, not content: do not rename existing identifiers or rewrite existing English docs just to comply, and match the surrounding language of any file you edit.

### Two-Step Confirmation First
- Never start the moment a requirement is stated or changed. Every new or modified requirement first passes through an understand-and-confirm step before any code is written. The only exception is trivial mechanical actions whose intent is obvious (e.g. `git push`, fixing a typo, a one-line rename).
- **Step one — reflect and surface, always in the open before building:**
  - Judge whether the request is sound: is it safe? is it efficient? does it fit the project's scale?
  - Consider whether a better approach exists than the one asked for.
  - State your full understanding of the request, your analysis of it (risks, tradeoffs, anything ill-advised), and your concrete recommendation.
- **Step two — act on the outcome:**
  - If the request is uncertain (multiple approaches with tradeoffs, may affect other features, or ill-suited to the project's scale), wait for the user's confirmation before modifying code.
  - If there is a clearly optimal and safe path, you may proceed without waiting for confirmation — but conspicuously notify the user that you are doing so up front, and on completion state plainly what you changed and why it was the better path.
  - Push back on any request that compromises security, correctness, or runtime efficiency, even when explicitly asked.

### Dry-Run Requests
- When the user's prompt contains "dry-run", treat the request as read-only: do not modify code, files, or configuration.
- Respond with two things: (1) your complete understanding of the request, and (2) a detailed, step-by-step explanation of how you would execute it.
- Make changes only after the user explicitly asks you to proceed.

### Simplicity First
- Write the minimum code that solves the stated problem; nothing speculative.
- No features, flags, config, or abstractions that weren't requested. Don't build "flexibility" for a future that isn't here.
- A single-use helper needs no abstraction — inline first, extract only on the second real caller.
- Validate untrusted input at the trust boundary, but don't add defensive handling for inputs that cannot occur.
- Adding a new third-party dependency needs the user's OK first; prefer the standard library and what's already in the project, and say why the dep is worth its cost.
- Before finishing, ask: would a senior engineer call this overcomplicated? If yes, cut it down.

### Scope Discipline
- Each task must stay within its stated scope. Do not add, refactor, or "improve" anything outside the current request.
- If the current task logically depends on another unbuilt feature, ask the user before implementing it. Never silently introduce adjacent functionality.
- Match the surrounding style and patterns even if you'd do it differently; don't reformat or refactor code your task doesn't touch.
- Clean up only the orphans your change creates (now-unused imports/vars/functions). Pre-existing dead code: flag it, don't delete it.

### Coding Standards
- **Source of truth & persistence**: no persistent state yet; the TUI model is the only state. Inputs come from argv and the TUI.
- **Architecture & boundaries**: single `package main` while small (`main.go` = CLI routing, `ui.go` = Bubble Tea model + tab shell, one file per panel such as `info.go` / `convert.go`, `ffmpeg.go` = format table, arg builder, background job runner; `ncm.go` = ncm2mp3 wrapper; `i18n.go` holds the EN→ZH table — wrap every new user-facing TUI string in `m.lang.t(...)` and add its translation). Split into packages only when a second real caller appears.
- **Language / framework conventions**: Go, `gofmt` + `go vet` clean. Bubble Tea v2 import paths are `charm.land/bubbletea/v2` and `charm.land/lipgloss/v2` (not the old `github.com/charmbracelet/*`). Subcommands open a TUI panel; they are shortcuts, not separate non-interactive CLIs. Tabs switch only with tab/shift+tab/1-3; arrows and hjkl belong to the active panel, which gets first refusal on every key.
- **Security boundary**: external tools are invoked via `exec.Command` with an argv slice — never through a shell, never with string-concatenated commands. User-supplied paths/URLs are passed as single arguments. Never execute downloaded content. Conversion never overwrites user files: `outputName` reserves the output with O_EXCL first (`name (N).ext` on collision), and only that reserved file is ever deleted on failure/cancel.
- **Verify**: `go vet ./... && go build -o mx .`, plus a tmux smoke run for UI changes.
- **Install**: `go build -o ~/.local/bin/mx .` (on PATH, survives Go upgrades; avoid `go install`, whose GOBIN is the mise per-version dir). Rebuild after changes.

### Secrets & Sensitive Data
- Never hardcode secrets or credentials; read them from environment/config. Never print, log, or echo secrets, tokens, or PII.
- Keep the project's seeded secrets-deny rules (covering `.env*`, `secrets/**`, and key files) in place.

### Verify & Commit
- Before coding, restate the task as a concrete, checkable success criterion rather than "make it work", and scale testing to the work's risk.
- After any code change, run the project's verification (e.g. type-check / build / lint) across all affected packages.
- If verification fails, fix the errors first, then verify again.
- After verification passes, automatically commit files that were modified by the agent in the current task and belong to the current task.
- Before committing, inspect dirty files and separate current-task agent edits from unrecognized dirty files. Do not include unrecognized dirty files in commits unless the user explicitly asks to include them.
- If a dirty file's ownership or task relevance cannot be determined safely, stop and ask the user before committing.
- Group commits by coherent change unit. Do not push unless explicitly requested.
- Branching: per-feature work lands on a `feat/*` branch that merges into the mainline. Keep merged `feat/*` branches as historical archives — never delete them (local or remote).
- Commit message format (Conventional Commits):
  ```
  type(scope): short summary

  Detailed description of what changed and why.
  ```
  Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `ci`, `perf`.

### Session Handoff
- `notice.md` files are scoped by directory and record durable handoff information for the next agent.
- At the start of a task, read the relevant `notice.md` files (the root one and those in the directories you'll work in) before making changes.
- Root `notice.md` records global, cross-package, or cross-task project information.
- App/package notices record durable facts for that app/package: architecture, contracts, workflows, credentials, known limitations, and package-specific gotchas.
- Update the relevant `notice.md` only when the session creates or discovers information that remains useful beyond the current task or conversation. Do not record pure Q&A, routine progress, temporary decisions, or workflow session/journal details there.
- Ensure any `notice.md` you touch remains accurate and up-to-date within its directory scope.

### End-of-Session Summary
- At the end of each conversation, output a brief summary in 中文 (Chinese). This summary is user-facing.
  ```
  ## Summary
  - **Done**: work completed this round
  - **Key decisions**: tag each [user] or [self], explain the decision
  - **Commits**: list this round's commits (hash, message, files); note any dirty files left out and why
  - **Open/known risks**: issues introduced or discovered this round (omit if none)
  - **Suggested next steps**: 1-3 concrete actionable items
  ```
</CORE_RULES>

<PROFILE id="tool">
**Profile: lightweight tool / script** — one-off scripts, small CLIs, utilities.
- Scale verification to the risk: a documented manual check or a smoke test can be enough; don't build a test harness a small tool doesn't warrant.
- Skip layered architecture and speculative abstraction; keep it a small, readable unit.
</PROFILE>
