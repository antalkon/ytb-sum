# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- `install.sh` installs into a writable directory already on `PATH`
  (e.g. `/opt/homebrew/bin`) so the command works immediately; otherwise it
  adds `~/.local/bin` to the shell profile. It also installs missing `yt-dlp`
  and `ffmpeg`, and removes copies left by earlier installs.
- `install.ps1` makes `ytb-sum` available in the current session and installs
  `yt-dlp` / `ffmpeg` with winget.

## [0.1.0] - 2026-10-24

### Added

- `ytb-sum <url>`: summarize a YouTube video into TL;DR, key points,
  timestamped sections, takeaways and mentioned resources.
- Styled terminal output (lipgloss + glamour) and `<video title>.md` saved to
  the current directory without overwriting existing files.
- OpenRouter support with `fast` / `balanced` / `best` presets, any model via
  `--model`, automatic fallback model and per-run cost in the footer.
- Any OpenAI-compatible endpoint via `--base-url` (Ollama, LM Studio, …).
- Strict JSON-schema output with one automatic repair round.
- Map-reduce condensing for transcripts beyond ~300k tokens.
- `--audio`: transcription of videos without subtitles via Gemini.
- Configuration via flags, environment, `~/.config/ytb-sum/config.yaml` and
  the macOS Keychain; `config init|show|path` subcommands.
- No on-disk leftovers: in-memory subtitles, `--no-cache-dir`, temp cleanup on
  success, error and Ctrl+C, process-tree kill on cancellation.
- Installers for macOS/Linux (`install.sh`) and Windows (`install.ps1`),
  `.deb`/`.rpm`/`.apk` packages, binaries for 6 OS/arch targets.

[0.1.0]: https://github.com/antalkon/ytb-sum/releases/tag/v0.1.0
