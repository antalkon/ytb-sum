<div align="center">

# ytb-sum

**Саммари любого видео с YouTube прямо в терминале — и сразу в Markdown.**

[![CI](https://github.com/antalkon/ytb-sum/actions/workflows/ci.yml/badge.svg)](https://github.com/antalkon/ytb-sum/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/antalkon/ytb-sum)](https://github.com/antalkon/ytb-sum/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/antalkon/ytb-sum)](https://goreportcard.com/report/github.com/antalkon/ytb-sum)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) · Русский

</div>

```text
$ ytb-sum https://youtu.be/aircAruvnKk
  ✔ Fetching video info 2.1s
  ✔ Downloading subtitles 0.4s
  ✔ Summarizing with anthropic/claude-sonnet-5.5 19.8s

  ╭──────────────────────────────────────────────────────────────╮
  │ ▶ But what is a neural network? | Deep learning chapter 1    │
  │ 3Blue1Brown  ·  18:40  ·  2017-10-05                         │
  │ https://www.youtube.com/watch?v=aircAruvnKk                  │
  ╰──────────────────────────────────────────────────────────────╯

  ▍Кратко
  Нейронная сеть для распознавания цифр — это цепочка слоёв нейронов …

  ▍Ключевые тезисы
  • Нейрон в сети — это просто «хранилище» числа от 0 до 1 …

  ▍Содержание
  › 0:00 · Распознавание рукописной цифры
  › 2:42 · Что такое нейроны
  …
  anthropic/claude-sonnet-5.5  ·  subs: en  ·  8.1k→2.8k tok  ·  $0.0440  ·  25.4s
  ✔ Saved But what is a neural network Deep learning chapter 1.md
```

Полный пример сохранённого файла: [docs/example-ru.md](docs/example-ru.md), а также его версия в виде веб-страницы: [docs/example-ru.html](docs/example-ru.html).

## Возможности

- **Чёткое структурированное саммари**: кратко, ключевые тезисы, разделы по хронологии с кликабельными таймкодами, выводы и упомянутые ресурсы.
- **Красивый вывод в терминал**: цвета и стили, подстройка под тёмную и светлую тему. При выводе в pipe или с `--no-color` печатается чистый Markdown.
- **Сохраняет `<название видео>.md`** в текущую папку. Существующие файлы не перезаписываются, к имени добавляется ` (2)`.
- **Веб-страница**: `--html` дополнительно сохраняет красивый самодостаточный `.html` (тёмная и светлая тема, навигация по разделам, кликабельные таймкоды) и открывает его в браузере. Команда `ytb-sum html <файл.md>` делает то же для любого сохранённого саммари.
- **Любая модель через [OpenRouter](https://openrouter.ai)** или любой OpenAI-совместимый API (Ollama, LM Studio…). Если основная модель недоступна, запрос автоматически уходит на запасную.
- **Любой язык саммари** (`--lang`, по умолчанию `ru`) независимо от языка видео.
- **Видео без субтитров**: `--audio` расшифровывает звуковую дорожку через Gemini.
- **Длинные видео**: до ~1 млн токенов за один проход, дальше транскрипт сжимается по частям (map-reduce).
- **Не оставляет мусора**: субтитры загружаются сразу в память, yt-dlp работает с `--no-cache-dir`, временное аудио удаляется даже при Ctrl+C, дочерние процессы завершаются вместе со всем деревом.

## Установка

Для работы нужен **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, а для `--audio` ещё и **ffmpeg**.

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.sh | sh
```

Больше ничего не нужно. Скрипт:

- определяет ОС и архитектуру, проверяет SHA-256;
- ставит бинарник в папку, которая уже есть в `PATH` и доступна на запись (например, `/opt/homebrew/bin`), поэтому `ytb-sum` работает сразу. Если такой нет, ставит в `~/.local/bin` и дописывает её в профиль shell (`.zshrc`, `.bashrc`, fish);
- доустанавливает `yt-dlp` и `ffmpeg` через Homebrew, apt, dnf, pacman или apk.

Настройки: `YTB_SUM_VERSION=v0.1.0`, `YTB_SUM_INSTALL_DIR=…`, `YTB_SUM_NO_DEPS=1`, `YTB_SUM_NO_MODIFY_PATH=1`.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.ps1 | iex
```

Добавляет `ytb-sum` в пользовательский `PATH` (команда работает сразу в том же окне) и при необходимости ставит `yt-dlp` и `ffmpeg` через winget.

### Пакеты для Linux

К каждому [релизу](https://github.com/antalkon/ytb-sum/releases/latest) приложены `.deb`, `.rpm` и `.apk`:

```bash
sudo dpkg -i ytb-sum_*_linux_amd64.deb
```

### Через Go

```bash
go install github.com/antalkon/ytb-sum/cmd/ytb-sum@latest
```

### Вручную

Скачайте архив под свою платформу со страницы [Releases](https://github.com/antalkon/ytb-sum/releases/latest), распакуйте и положите `ytb-sum` в любую папку из `PATH`. Сборки есть для Linux, macOS и Windows на amd64 и arm64.

## Быстрый старт

1. Получите ключ на <https://openrouter.ai/keys>.
2. Сделайте его доступным утилите, любым из способов:

   ```bash
   export OPENROUTER_API_KEY=sk-or-...                          # например, в ~/.zshrc
   security add-generic-password -a "$USER" -s ytb-sum -w       # Связка ключей macOS
   ```

3. Запустите:

   ```bash
   ytb-sum https://www.youtube.com/watch?v=aircAruvnKk
   ```

## Использование

```text
ytb-sum <youtube-url|video-id> [флаги]
```

Понимает ссылки `watch?v=`, `youtu.be/`, `/shorts/`, `/live/`, `/embed/`, `music.youtube.com` и голые 11-символьные id.

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-q, --quality` | `balanced` | Пресет модели: `fast`, `balanced`, `best` |
| `-m, --model` | — | Любая модель OpenRouter, перекрывает `--quality` |
| `--fallback-model` | `openai/gpt-6-sol` | Запасная модель, если основная упала |
| `-l, --lang` | `ru` | Язык саммари (`en`, `de`, … или название, например `"Brazilian Portuguese"`) |
| `--length` | `medium` | `short`, `medium`, `long` |
| `--sub-lang` | язык видео | Предпочитаемые субтитры |
| `-a, --audio` | выкл | Расшифровать аудио, если субтитров нет |
| `--audio-model` | `google/gemini-3.8-flash` | Модель для `--audio` |
| `-o, --output` | текущая папка | Куда сохранить `.md` |
| `--no-file` | выкл | Только вывод в терминал |
| `--html` | выкл | Дополнительно сохранить `.html` и открыть в браузере |
| `--no-open` | выкл | С `--html`: сохранить страницу, но не открывать браузер |
| `--no-color` | выкл | Без цветов (также учитывается `NO_COLOR`) |
| `--cookies-from-browser` | — | Куки браузера для yt-dlp (видео 18+ и для спонсоров), например `chrome` |
| `--base-url` | OpenRouter | Любой OpenAI-совместимый endpoint |
| `--api-key` | — | Лучше использовать переменную окружения или Связку ключей |
| `--timeout` | `10m` | Общий таймаут |
| `-v, --verbose` | выкл | Печатать этапы, даже если вывод не в терминал |

### Пресеты моделей

| Пресет | Модель | ≈ цена за час видео |
|---|---|---|
| `fast` | `anthropic/claude-haiku-5.5` | ~$0.004 |
| `balanced` | `anthropic/claude-sonnet-5.5` | ~$0.07 |
| `best` | `anthropic/claude-opus-5.5` | ~$0.14 |

Фактическая стоимость запуска, по данным OpenRouter, печатается в нижней строке вывода.

### Примеры

```bash
ytb-sum -q fast --length short https://youtu.be/dQw4w9WgXcQ   # быстро и дёшево
ytb-sum -l en -o ~/Notes/youtube <url>                         # на английском, в папку заметок
ytb-sum -m deepseek/deepseek-v4.1-flash <url>                  # любая модель OpenRouter
ytb-sum --audio <url>                                          # видео без субтитров
ytb-sum --no-file --no-color <url> | less                      # чистый Markdown в pipe
ytb-sum --html <url>                                           # .md + .html, сразу в браузере
ytb-sum html "Название видео.md"                               # веб-страница из готового саммари
ytb-sum --base-url http://localhost:11434/v1 -m qwen3 <url>    # локальная Ollama
```

## Настройка

Приоритет: **флаги → переменные окружения → файл конфига → значения по умолчанию**.

```bash
ytb-sum html <файл.md> [-o page.html] [--no-open]   # саммари в Markdown → веб-страница
ytb-sum config init   # создать ~/.config/ytb-sum/config.yaml (права 600)
ytb-sum config show   # итоговые настройки, ключ скрыт
ytb-sum config path
```

| Переменная | Значение |
|---|---|
| `OPENROUTER_API_KEY` / `YTB_SUM_API_KEY` | API-ключ |
| `YTB_SUM_MODEL`, `YTB_SUM_QUALITY`, `YTB_SUM_AUDIO_MODEL` | Модели |
| `YTB_SUM_LANG`, `YTB_SUM_OUTPUT_DIR`, `YTB_SUM_BASE_URL` | То же, что флаги |
| `YTB_SUM_CONFIG` | Другой путь к конфигу |
| `NO_COLOR` | Отключить цвета |

Где утилита ищет ключ, по порядку: `--api-key` → `OPENROUTER_API_KEY` → `YTB_SUM_API_KEY` → Связка ключей macOS (сервис `ytb-sum`) → файл конфига.

## Как это работает

```text
URL ─▶ yt-dlp --dump-json ─▶ метаданные, главы, описание          (в памяти)
     └▶ субтитры (ручные > оригинальные авто) ─▶ HTTP ─▶ транскрипт  (в памяти)
        └ нет субтитров + --audio ─▶ временное аудио ─▶ ffmpeg, куски по 20 мин ─▶ Gemini ─▶ транскрипт
транскрипт + метаданные ─▶ LLM со строгой JSON-схемой ─▶ Summary
Summary ─▶ терминал (lipgloss + glamour)  и  <название>.md
```

Модель отвечает по строгой JSON-схеме. Поэтому терминал и Markdown-файл собираются из одних и тех же данных, а некорректный ответ модели автоматически исправляется повторным запросом.

**Без мусора.** На диск пишется только итоговый `.md`. Временные папки создаются только для `--audio` (и в редком запасном способе загрузки субтитров). Они удаляются через `defer` при успехе, ошибке и Ctrl+C. Если процесс убили через `kill -9`, оставшиеся папки удалятся при следующем запуске.

## Частые проблемы

| Проблема | Решение |
|---|---|
| `yt-dlp not found` | Установите yt-dlp и проверьте, что он в `PATH` |
| `Sign in to confirm you're not a bot` / видео 18+ | `--cookies-from-browser chrome`; обновляйте yt-dlp (`yt-dlp -U` / `brew upgrade yt-dlp`) |
| `this video has no subtitles` | Запустите с `--audio` |
| `401` | Проверьте ключ: `ytb-sum config show` |
| `402` | Пополните баланс OpenRouter |
| `No endpoints found that can handle the requested parameters` | Модель не поддерживает structured output, выберите другую `--model` |

## Разработка

```bash
make test        # go test -race ./...
make lint        # golangci-lint
make build       # ./bin/ytb-sum
make snapshot    # сборка goreleaser под все платформы в ./dist
```

Релизы собираются автоматически. Запушьте тег `vX.Y.Z`, и [Release workflow](.github/workflows/release.yml) прогонит тесты, а GoReleaser опубликует бинарники, пакеты и контрольные суммы.

## Лицензия

[MIT](LICENSE)
