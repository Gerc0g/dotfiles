# Charm — основной стек консольной/TUI-разработки

Это наш дефолтный стек для терминальных интерфейсов и «глянцевых» CLI. Когда
делаем TUI/CLI — берём отсюда, не изобретаем своё. LSP-индексация всего стека
живёт в `tools/charm-stack/` (см. раздел «LSP» внизу).

Дом: <https://charm.land> (редирект с charm.sh) · GitHub: <https://github.com/charmbracelet>

---

## Библиотеки (на чём пишем)

| Либа | Что | Import | Когда брать | Doc |
|------|-----|--------|-------------|-----|
| **Bubble Tea** | Фреймворк приложения (Elm-арх: `Model`/`Init`/`Update`/`View`, `tea.Cmd`/`tea.Msg`) | `github.com/charmbracelet/bubbletea` | Любой интерактивный TUI с состоянием, клавиатурой, мышью | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/bubbletea) |
| **Bubbles** | Готовые виджеты: `table`, `list`, `viewport`, `spinner`, `progress`, `textinput`, `textarea`, `paginator`, `help`, `key`, `timer`, `stopwatch`, `filepicker`, `cursor` | `github.com/charmbracelet/bubbles/<widget>` | Не писать виджеты руками | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/bubbles) |
| **Lip Gloss** | Стили: цвета, рамки, padding/margin, width/height, align, `JoinHorizontal/Vertical`, `Place`; саб-пакеты `table`/`list`/`tree` (статический рендер) | `github.com/charmbracelet/lipgloss` | Вся отрисовка/верстка | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/lipgloss) |
| **Huh** | Формы/промпты/визарды: `Input`, `Text`, `Select`, `MultiSelect`, `Confirm`, `Note`, `FilePicker`; темы, валидация, accessible-режим | `github.com/charmbracelet/huh` | Интерактивный ввод, мастера настройки | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/huh) |
| **Glamour** | Markdown → стилизованный терминал | `github.com/charmbracelet/glamour` | Показ md (help, README, отчёты) | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/glamour) |
| **Wish** | Bubble Tea-приложения по SSH (`wish.NewServer`, `bubbletea.Middleware`, `activeterm`, `accesscontrol`, `logging`) | `github.com/charmbracelet/wish` | Раздать TUI по SSH, git-сервер | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/wish) |
| **Log** | Структурный цветной логгер (`log.New`, уровни, `With`, JSON/logfmt/text, `SetStyles` через Lip Gloss) | `github.com/charmbracelet/log` | Логи в CLI/демонах | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/log) |
| **Harmonica** | Физическая анимация (пружины: `NewSpring`, `Update`) | `github.com/charmbracelet/harmonica` | Плавные переходы/бары | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/harmonica) |
| **Fang** | Обёртка над Cobra: стильные help/errors, `--version`, manpages, completions (`fang.Execute`) | `github.com/charmbracelet/fang` | Новый CLI на Cobra с фирменным видом | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/fang) |
| **x/ansi** | ANSI-aware утилиты (`ansi.Truncate`, `ansi.StringWidth`, `ansi.Strip`) | `github.com/charmbracelet/x/ansi` | Безопасная обрезка/измерение стилизованных строк | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/x/ansi) |
| **colorprofile** | Даунсэмплинг цвета под возможности терминала | `github.com/charmbracelet/colorprofile` | Совместимость цветов | [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/colorprofile) |

## Инструменты (готовые CLI, не библиотеки)

| Тулза | Что | Польза у нас | Doc |
|-------|-----|--------------|-----|
| **Gum** | Charm-интерактив в shell-скриптах: `choose`, `filter`, `input`, `write`, `confirm`, `spin`, `table`, `style`, `join`, `format`, `pager`, `file`, `log` | Глянцевые куски в наших zsh/bash-скриптах без Go | [repo](https://github.com/charmbracelet/gum) |
| **VHS** | Запись терминала по `.tape`-скриптам → gif/mp4/webm/png (`Output`, `Set`, `Type`, `Enter`, `Sleep`, `Hide/Show`, `Screenshot`) | Демки/доки TUI воспроизводимо | [repo](https://github.com/charmbracelet/vhs) |
| **Freeze** | Скриншоты кода/вывода терминала → PNG/SVG/WebP (темы, тени, рамки, `--execute`, `--interactive`) | Картинки для доков; `tmux capture-pane \| freeze` | [repo](https://github.com/charmbracelet/freeze) |
| **Glow** | TUI-ридер markdown (на Glamour) | Чтение доков/вики | [repo](https://github.com/charmbracelet/glow) |
| **Mods** | LLM в пайпах CLI | AI-помощь в шелле | [repo](https://github.com/charmbracelet/mods) |
| **Crush** | AI-агент в терминале (mac/linux/win/bsd) | Альт. агент | [repo](https://github.com/charmbracelet/crush) |
| **Skate** | Персональный key-value store с CLI | Заметки/состояние между сессиями | [repo](https://github.com/charmbracelet/skate) |
| **Soft Serve** | Самохостящийся git-сервер с TUI | Git по SSH | [repo](https://github.com/charmbracelet/soft-serve) |
| **Wishlist** | SSH-директория/jump-host | Каталог SSH-эндпоинтов | [repo](https://github.com/charmbracelet/wishlist) |
| **Pop / Melt** | Pop — почта из терминала; Melt — бэкап SSH-ключа в мнемонику | По ситуации | [pop](https://github.com/charmbracelet/pop) · [melt](https://github.com/charmbracelet/melt) |

---

## ВАЖНО: версии v1 vs v2

Доки на charm.land сейчас описывают **Lip Gloss v2** (`charm.land/lipgloss/v2`,
`Blend1D`/`Blend2D`, `LightDark(hasDarkBackground)`, `BorderForegroundBlend`).
**Наш `status-tui` на v1** — другой import path и API:

| Пакет | У нас (pin) | Замечание |
|-------|-------------|-----------|
| lipgloss | `github.com/charmbracelet/lipgloss` **v1.1.0** | без `Blend*`, есть `AdaptiveColor`, `table`/`list`/`tree` |
| bubbletea | **v1.3.10** | `tea.KeyMsg` (в v2 — `tea.KeyPressMsg`) |
| bubbles | **v1.0.0** | `progress.ViewAs`, `table.New` |

Не копировать примеры с charm.land дословно в v1-код — проверять сигнатуры через
`go doc` / LSP. Миграцию на v2 делать отдельным заходом, осознанно.

## Подводные камни (проверено на status-tui)

- **Заливка фона + вложенные ANSI-reset.** Lip Gloss не заливает `Background`
  равномерно за уже-стилизованным контентом (бары/чипы/clamp вставляют reset),
  фон «протекает» прямоугольными пятнами. Правило: либо фон только на самом
  внешнем слое и без вложенных стилей с reset, либо **не использовать заливку
  карточек** (только цветные рамки). См. `tools/status-tui`.
- **Списки: `lipgloss/table`, не `bubbles/table`.** `bubbles/table` интерактивный
  (viewport, курсор, header+разделитель = 2 лишние строки) и красит ячейки одним
  стилем. `lipgloss/table` — статичный render-only с `StyleFunc(row,col)` для
  per-cell цвета. Берём его.
- **`lipgloss/table` v1.1.0 off-by-one.** При `Border(HiddenBorder())` + всех
  `Border*(false)` теряется последняя строка (8 → 7). Обход: `Border(lipgloss.Border{})`
  `.BorderColumn(false).BorderRow(false)` рендерит 8 строк, но добавляет пустые
  строки сверху/снизу (→10) — срезать их вручную. См. `listTable()` в `status-tui`.
- **Статичный рендер виджетов.** `progress.ViewAs(pct)` рендерит бар без
  анимации/спринга — то, что нужно для auto-refresh дашборда.
- **Клампить всё через `ansi.Truncate`**, иначе строка шире карточки заворачивается
  и ломает горизонтальный `JoinHorizontal`.

## Карта выбора

- Нужен интерактив с состоянием → **Bubble Tea** + **Bubbles** + **Lip Gloss**.
- Только ввод/мастер → **Huh** (standalone `form.Run()` или как `tea.Model`).
- Глянец в shell-скрипте → **Gum** (Go не писать).
- Показать markdown → **Glamour** (либа) / **Glow** (готовый ридер).
- Раздать TUI по сети → **Wish**.
- Демка/скриншот для доков → **VHS** (gif) / **Freeze** (png).
- Новый Cobra-CLI с фирменным видом → **Fang**.

---

## LSP по стеку («LSP-сервер по библиотекам»)

`tools/charm-stack/` — модуль-манифест: один `go.mod` тянет **весь** собираемый
стек, а `stack.go` ссылается по одному символу на каждую либу. Благодаря этому
`gopls` индексирует всё сразу.

- **В редакторе (VS Code Go / любой gopls-клиент):** работает из коробки —
  hover, автодополнение, go-to-def по любому символу Charm. `gopls` установлен в
  `~/go/bin/gopls`.
- **В харнесе агента (LSP-инструмент):** нужен включённый плагин
  `gopls@claude-code-lsps` в `enabledPlugins` (`~/.claude-new/settings.json`) и
  **рестарт** Claude Code. До этого использовать `go doc <pkg> [Symbol]` — даёт
  точные сигнатуры всегда.
- **Всегда доступно:** `cd tools/charm-stack && go doc github.com/charmbracelet/huh NewForm`.

Обновлять стек: `cd tools/charm-stack && go get -u ./... && go mod tidy`.
