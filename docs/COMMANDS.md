# Personal Platform Commands

Single source of truth для custom commands.

Format: `## <command>` + `**Что:**` `**Запуск:**` `**Файлы:**` etc.
Группируется по `### Category` (порядок секций определяет порядок в `?`).

---

### Discovery

## help
**Что:** Список команд по категориям. `help <cmd>` для деталей.
**Запуск:** `help` или `help <cmd>`

## ?
**Что:** Короткий alias для `help`.
**Запуск:** `?` или `? <cmd>`

---

### System status

## status
**Что:** Красивый full-screen TUI для текущего pressure: RAM/swap/compressor/wired, active agent slots, tmux-сессии и top offenders. Показывает `OK/WARN/CRITICAL` относительно локального лимита активных агентов.
**Запуск:**
- `status` — открыть live dashboard, auto-refresh каждые 5 сек.
- `status --once` — напечатать snapshot без TUI.
**Настройки:** `STATUS_LOCAL_AGENT_LIMIT=2` — локальный лимит Claude+Codex процессов для alerting.
**Выход:** `q`, `Esc` или `Ctrl+C`. Refresh вручную — `r`.

---

### Main flow

## agent-workspace
**Что:** управление изолированными git worktree под задачи агента. Логика в ядре (`hq workspace`, алиас `hq ws`); shell-обёртка нужна только чтобы перейти в каталог и открыть редактор. Ярлыки продуктов (`agents synapse ticket-quality`, `healler epic-04`) вызывают это автоматически.
**Запуск:**
- `agent-workspace start <co> <prod> <repo> <task>` — создать worktree, перейти в него и открыть в редакторе.
- `agent-workspace open <wt-path> | <co> <prod> <repo> <id>` — вернуться в существующий worktree (нового id не создаётся).
- `agent-workspace list` — список worktree с id/task/branch/state/dirty.
- `agent-workspace stale [days]` — worktree без коммитов N+ дней (по умолчанию 3) для ручного разбора; `start` подсказывает их количество.
- `agent-workspace ready <co> <prod> <repo> <id>` — пометить clean+pushed worktree готовым к уборке.
- `agent-workspace remove <co> <prod> <repo> <id>` — удалить clean worktree по короткому id.
- `agent-workspace cleanup [--days N] [--dry-run]` — удалить только clean + pushed + ready/merged worktree.
- `agent-workspace prune-branches [--dry-run]` — удалить ветки `agent/*`, не привязанные ни к одному worktree и при этом merged в base или полностью pushed.
**Редактор:** вход в worktree открывает его в VS Code. `HQ_EDITOR` меняет бинарник, `HQ_OPEN_EDITOR=0` отключает открытие целиком — так выставлено на сервере, где GUI нет.
**Git:** base branch = `dev`, иначе `main`; каталог = короткий id; ветка = `agent/<task>-<id>`; `user.name`/`user.email` берутся из конфига компании.
**Salvage:** перед удалением worktree незакоммиченные `docs/epics/*.md` спасаются в WikiPedik `repos/<repo>/_salvage/<id>/`. Изменённые tracked-файлы по-прежнему блокируют удаление.
**Автоудаления нет:** worktree удаляется только явно (`remove`) или через `cleanup` (нужен флаг ready и 7 дней). Автоматическая уборка при старте когда-то дважды снесла живую работу и была убрана.
## hq-ctx
**Что:** Определить контекст текущего каталога в модели мира: компания/продукт/репозиторий/worktree, из любой глубины вложенности. Единственный резолвер контекста — скрипты и обёртки не разбирают пути сами.
**Запуск:**
- `hq ctx` — человекочитаемый ответ (ref, kind, путь).
- `hq ctx --plain` — `key=value` по строке для скриптов: kind/company/product/repo (+ id/task/branch/state в worktree).
- `hq ctx <dir>` — контекст произвольного каталога.

## hq-editor-sync
**Что:** Синхронизировать модель мира (продукты, репозитории, активные worktree) с VS Code Project Manager (`alefragnani.project-manager`). Project Manager — зеркало платформы: управляемые записи помечены тегом `dotfiles`, протухшие и внешние записи удаляются. Логика в ядре (`core/editor`); ярлык `vscode-projects-sync` сохранён. Автоматически вызывается при `hq workspace start`.
**Naming:** repo entries называются коротко (`synapse`), product entries — `Product: company/product`, worktree entries — `repo @ id · task`; company/product/state/branch доступны через tags.
**Запуск:**
- `hq editor sync` — добавить/обновить product + repo + worktree entries, убрать протухшие; перед записью делает backup.
- `hq editor sync --dry-run` — показать изменения без записи.
- `hq editor sync --check` — exit 1, если файл не синхронизирован.
- `hq editor sync --repos-only | --products-only | --no-worktrees` — сузить набор записей.
- `hq editor sync --preserve-external` — сохранить проекты, добавленные вручную вне dotfiles.
- `hq editor sync --no-prune | --no-backup | --project-file <path>` — тонкая настройка.
**Файл:** `~/Library/Application Support/Code/User/globalStorage/alefragnani.project-manager/projects.json`

## hq-commit
**Что:** локальный атомарный commit одного logical change. Логика в ядре; `agent-commit.sh` остался шимом. Проверяет Conventional Commit префикс, git_email компании (включая worktree), гоняет make verify/test.
**Запуск:**
- `hq commit "feat(scope): русское описание" -- <explicit paths>` — commit после завершённого logical change.
- `hq commit --no-verify ...` — пропустить make verify/test (аналог `AGENT_GIT_VERIFY=0`).
**Правила:** explicit paths only; no `git add .`, no force, no rebase, не push.

## hq-finish
**Что:** финал agent-задачи: verify/test, push branch, открыть draft PR/MR в integration branch, записать review metadata в `.agent-workspace`. Логика в ядре; `agent-finish.sh` и `agent-task-push.sh` остались шимами.
**Запуск:**
- `hq finish` — основной финальный flow.
- `hq finish --base dev --title "feat(scope): описание"` — override base/title.
- `hq finish --no-review [--allow-dirty]` — низкоуровневый fallback push без PR/MR (бывший agent-task-push); интеграционные ветки защищены `AGENT_ALLOW_INTEGRATION_PUSH=1`.
- `hq finish --ready-without-review` — пометить ready, если ревью не открылось.
**Правила:** push не на каждый commit; merge не выполняется автоматически.

## routine
**Что:** Фоновые задачи платформы как отдельная сущность: реестр, запуск, гварды стоимости, единый журнал. Заводится потому, что фоновая работа расползается по механизмам и её никто не видит целиком, а часть задач запускает агента — забытая задача жжёт подписку молча.
**Запуск:**
- `hq routine` — реестр: имя, расписание, последний исход, для агентских — счётчик «тратит токены, N/M за сутки».
- `hq routine run <имя> [--force]` — запустить сейчас; то же самое вызывает launchd по расписанию. `--force` обходит гварды.
- `hq routine logs [имя] [-n N]` — единый журнал фоновой работы (`~/.cache/hq/routines.log`).
- `hq routine enable|disable <имя>` — выключить задачу, не удаляя её историю и расписание.
**Встроенные задачи:**
- `transcript-scrub` (03:30) — вычистить секреты из транскриптов агентов.
- `memory-drain` (каждые 6 ч) — разобрать инбокс самого запущенного репозитория. Агентская, потолок 4 запуска в сутки.
- `vault-sync` (каждые 6 ч) — закоммитить и запушить вольт. Нужна потому, что фоновая работа оставляет вольт изменённым, но незафиксированным: скраббер правит дайджесты и уходит, а scope-атомарные коммиты разбора никогда не пушатся. Уступает работающему разбору по общему замку — иначе закоммитила бы страницу, которую куратор пишет прямо сейчас.

**Типы:**
- `internal` — работа ядра, стоит только электричества (скраббер транскриптов).
- `agent` — запускает агента и тратит подписку (разбор памяти). Потолок запусков за сутки обязателен: если не задан явно, применяется дефолтный.
**Свои задачи:** `~/.config/hq/routines.yaml` — список записей. Платформа даёт расписание, гварды и запись исхода; вы даёте намерение.
```yaml
- name: calendar-digest
  about: утренний разбор календаря
  at: "08:00"              # либо every: 6h
  prompt: |
    Посмотри мой календарь на сегодня и сделай краткий дайджест.
  dir: ~/Desktop/WikiPedik/dev
  agent: codex             # или claude
  max_runs_per_day: 2
```
**Расписание:** держит launchd; плисты генерируются из реестра в `~/Library/LaunchAgents/com.gerc0g.routine.*`. Править их руками бессмысленно — `hq setup` перезапишет. Удалили routine из конфига — `hq setup` снимет и её задачу.
**Запуск от launchd** идёт через `bin/hq-cron`: вольт лежит под `~/Desktop`, это территория TCC, а доступ выдаётся конкретному пути. У `bin/hq` уже стоит явный запрет от диалога, на который фоновая задача не могла ответить.
**Наблюдаемость:** `hq doctor` показывает `routines` (расписания соответствуют реестру) и `routine-pulse` (задачи действительно отрабатывают). Расписание — это заявление, а не доказательство: дважды фоновая работа умирала молча при полностью здоровой декларации.

## wikipedik
**Что:** Вольт и его личные зоны (`hq wikipedik`): `dev` принадлежит памяти проектов (`hq wiki`), `research` — зона обучения, `brand` — контент (не наполнена). Команда `wikipedik` — единственная дверь: переходит в зону и запускает агента, дальше работа идёт разговором.
**Запуск:**
- `wikipedik [зона]` — перейти в зону (по умолчанию `research`), показать пульс и запустить codex. `WIKIPEDIK_AUTOSTART=0` — только перейти.
- `hq wikipedik` — обзор зон: файлы, последний коммит, незакоммичено/незапушено.
- `hq wikipedik sync "кратко по-русски"` — закоммитить ВЕСЬ вольт одним коммитом (секрет-скан диффа) + pull --rebase + push. Префикс Conventional Commit добавляется автоматически (`docs(vault): …`). Этим агент завершает каждое изменение вольта — правило зашито в контракт зоны и в чекпойнт сессии; при старте сессии пульс показывает, если что-то осталось незакоммиченным.
**Правила вольта:**
- Пишем на русском; English content — по явной просьбе, отдельной копией ` - EN.md`.
- Git покрывает весь vault: `dev`, зоны и Obsidian metadata; push — руками или `hq wikipedik sync`.
- Commit message: English Conventional Commit type/scope + русское описание.

## research
**Что:** Зона обучения в вольте. Единица — тема, которую пользователь изучил сам; одна тема = один растущий файл. Агент (codex) — писарь и редактор конспектов: пользователь работает ТОЛЬКО через чат, команды `hq research …` вызывает агент. Контракт — `research/AGENTS.md` (+ симлинк `CLAUDE.md`).
**Структура зоны:** `topics/<домен>/` (ml, math, cs, quant — плоско; папка это только место, где лежит файл), `maps/` (карты тем вместо папок-подтем), `sources/` (паспорта материалов), `raw/` (оригиналы, неприкосновенны, + `raw/inbox/`), `index.md` (генерируется), `log.md`.
**Формат конспекта:** обязательные шапка (frontmatter + H1 + «В двух словах») и хвост («Связи», «Источники»), свободная середина под материал; формулы в LaTeX; цитаты с точностью до страницы/раздела.
**Инструменты агента:**
- `hq research` — пульс зоны: темы по доменам, статусы, качество.
- `hq research new <домен>/<slug> --title "Название"` — тема из шаблона.
- `hq research list [--domain ml] [--plain]` — что уже есть (проверка дублей).
- `hq research lint [--plain]` — качество: каркас, LaTeX, точность цитат, битые связи, дубли.
- `hq research index` — перегенерация `index.md` и покрытия во всех картах.
- `hq research next [область]` — что изучать дальше: первый пункт маршрута без конспекта. Карта области разрастается до сотен пунктов и превращается в справочник; очередь возвращает ей смысл плана.
- `hq research map <slug> --title "Название" --domain ml` — карта-роадмап области: цель, маршрут, пробелы, источники. Вложенность любая — разделы и подразделы внутри файла, папки под темы не заводятся.
- `hq research map-candidates` — теги с 3+ темами без карты.
- `hq research graph` — настроить граф Obsidian под учебную зону: показываются только `topics`, `maps`, `sources` (контракт агента, генерируемый индекс и журнал исключены — последние два стянули бы граф в звезду), темы покрашены по доменам (math синий, ml оранжевый, quant зелёный, cs фиолетовый, карты жёлтые, источники серые), стрелки связей включены, сироты видны намеренно. Прежние настройки сохраняются в `graph.json.bak.<время>`; после применения нужен перезапуск Obsidian.
**Верхняя карта:** `maps/roadmap.md` — все области списком, каждая ссылкой на свою карту. Карта области заводится, когда в неё входят; пока её нет, область считается неначатой. Карта, ссылающаяся на карту, подтягивает её числа (`◑ [[dl|DL]] · 1/3`), поэтому верхняя сводка складывает области, а не строки.
**Темы вне карт:** роадмап сам не расширяется — план это выбор, а не зеркало всего написанного; иначе знаменатель покрытия рос бы с каждым файлом и проценты потеряли бы смысл. Конспекты, которых нет ни в одной карте, показываются в `hq research` и в пульсе сессии, чтобы решение внести их в маршрут принималось, а не терялось.
**Связь карты и конспекта:** только по имени — `[[gradient-descent]]` находит тему по полному пути, slug, заголовку H1 или алиасу. Поэтому агент обязан заводить конспект под тем именем, что уже стоит в маршруте: назовёт иначе — пункт навсегда останется «не начато».
**Покрытие в картах:** статус каждого пункта вычисляется из конспекта (`solid` → `✓` закрыто, `growing` → `◐` неполно, файла нет → `·` не начато) и обновляется вместе с индексом. Руками не ставится: карта, которую можно подкрутить, перестаёт быть честной. Ссылка на неизученную тему в карте — не ошибка, линтер карты не проверяет.
**Контекст сессии:** SessionStart в зоне отдаёт агенту состояние — сколько тем, что менялось последним, проблемы линта, кандидаты в карты.
**Профиль агента:** изолированный `CODEX_HOME=~/.codex-research` (шаг setup `codex-zones`): свои скиллы, хуки, история и trust; `auth.json` и `plugins/` — симлинки в дефолтный профиль (один логин, один кэш плагинов). Из промпта убраны чужие контуры: блок скиллов 3203 → ~930 токенов. Шаблон конфига — `agent-profiles/codex-research/config.toml`, копируется один раз (дальше codex дописывает туда trust сам).
**Коммиты вольта — на агенте, со страховкой** (aider-подобно: коммит на завершённое логическое изменение):
1. **Агент коммитит сам** после каждого логического изменения: `hq wikipedik sync "добавил миноры и критерий ранга"` — коммит всего вольта + push. Обязанность прописана в контракте зоны и в чекпойнте сессии. Только агент знает смысл правки, поэтому имена коммитов — его работа.
2. **Хуки Stop и SessionEnd** → `hq wikipedik autosync --push`: если агент забыл, коммитят машинным сообщением (`создан X`, `дополнен Y`, `удалён Z`) и пушат; если коммитить нечего, но есть локальные коммиты — просто пушат.
3. **Выход из `wikipedik`** → то же самое; рубеж не зависит ни от модели, ни от доверия хукам.
Пошаговый хук `PostToolUse` был убран сознательно: он срабатывает сразу после правки файла и всегда опережал агента, из-за чего каждое сообщение выходило машинным.
**Диагностика:** `~/.cache/wikipedik-autosync.log` — по строке на каждый вызов страховки; последняя строка видна в `hq wikipedik` (иначе отказ хуков невидим).
**Доверие хукам:** codex требует подтверждать хуки заново после каждой их правки, и один пропущенный запрос молча выключает автокоммиты (так и случилось при первом тесте). Поэтому `wikipedik` запускает агента с `--dangerously-bypass-hook-trust` — хуки наши, лежат в `~/dotfiles` под git. Вернуть штатный запрос: `WIKIPEDIK_HOOK_TRUST=ask wikipedik`.

## wiki
**Что:** WikiPedik project-memory commands. Без аргументов запускает wiki product repo, с подкомандами управляет curator flow. Детерминированная логика в ядре (`hq wiki ...`, все подкоманды ниже — обёртки); LLM-половина остаётся кураторскими скиллами. Вольт резолвится через `WIKIPEDIK_ROOT` (дефолт `~/Desktop/WikiPedik`).
**Запуск:**
- `wiki` — создать worktree в `neurodesk/wiki/nrdsk_wiki` и перейти в него.
- `wiki sync <company[/product[/repo]]>` — curator drain: `_inbox.md` → `lessons.md` / `gotchas.md` / etc. через `inbox-drain`.
- `wiki sync --commit <scope>` — drain + локальный commit WikiPedik changes. Заранее dirty файлы разрешены только внутри указанного scope.
- `wiki sync --push <scope>` — drain + commit + push.
- `wiki drain [<scope>]` — тот же curator drain, но в автономном режиме, без подтверждений по каждой записи. Без аргумента скоуп берётся из текущего каталога (`~/dotfiles` → `_platform`, воркспейс → `<co>/<prod>/<repo>`).
- `wiki drain --check [<scope>]` — показать решение гвардов, ничего не запуская и не тратя токены.
- `wiki drain --sweep` — разобрать самый запущенный репо вольта, а не текущий. Бэклог копится там, где сессий не открывают.
- `wiki cron` — плановый прогон: проверить доступ к вольту, записать исход, сделать свип. Это то, что запускает launchd.
- `wiki drain --background --commit` — то, что вызывает SessionEnd-хук: проверить гварды и, если надо, отцепить разбор в фон.
- `wiki status <company[/product[/repo]]>` — отчёт по pending inbox, curated pages, synthesis candidates.
- `wiki synthesize <company/product>` — найти повторяющиеся lessons/gotchas и предложить product shared patterns.
- `wiki bootstrap <company> [product] [repo]` — создать WikiPedik skeleton + repo symlinks.
- `wiki-commit <scope>` — вручную закоммитить только changes внутри WikiPedik scope.
- `wiki autocommit` / `wiki-autocommit` — catch-all commit всего dirty вольта (без push, с secret-scan). Автоматического вызывающего сейчас НЕТ (запускать вручную; `--if-due` оставлен для будущего планировщика).
- `wiki-hot-refresh <co>[/<prod>[/<repo>]]` — детерминированно пересобрать `hot.md` из curated-страниц + дайджест «Binding rules» (fallback, если куратор пропустил свою фазу).
- `wiki rules-sync [<co>[/<prod>[/<repo>]]]` — материализовать правила вольта (`repos/<repo>/rules/`, `shared/rules/`) в path-scoped `.claude/rules/wiki-*` симлинки (основные чекауты + активные worktrees); правила без `paths:` отклоняются; удалённые из вольта — prune.
- `agent-session-digest.py --source claude|codex|all [--since D] [--until D] [--project substr]` — сжать JSONL-сессии в markdown-дайджесты для майнинга куратором (`10-wiki/sources/sessions/_digests/`); secrets редактируются, объём падает с MB до десятков KB.
- `wiki-git <args...>` — выполнить `git` внутри root vault `~/Desktop/WikiPedik`.
**Примеры:**
- `wiki sync neurodesk/wiki/nrdsk_wiki`
- `wiki sync --commit neurodesk/wiki/nrdsk_wiki`
- `wiki-git status`
- `wiki status neurodesk`
- `wiki synthesize neurodesk/agents`

**Доставка знаний в сессию:**
- `SessionStart` вкладывает `hot.md` один раз, при открытии сессии.
- `UserPromptSubmit` досылает то, что появилось потом: сессия живёт неделями, а куратор разбирает инбокс по нескольку раз в день. Каждая сессия помнит показанный ей `hot.md` и получает ТОЛЬКО новые строки; не изменилось ничего — не печатается ничего, и это обычный случай. Слепки лежат в `~/.cache/wikipedik/sessions/` и подчищаются по возрасту.
- Перевыкладывать `hot.md` целиком нельзя: за неделю это десятки копий одного индекса в контексте.

**Автоматический разбор:**
- Основной триггер — launchd-задача `com.gerc0g.wikipedik-drain` каждые 6 часов (`hq wiki cron --commit`). Единственный триггер, не зависящий от того, как вы работаете: сессионные хуки не срабатывают у того, кто сидит в одной сессии неделями.
- В плисте задан `PATH` с `/opt/homebrew/bin`: launchd отдаёт задаче голый `/usr/bin:/bin`, и без этого прогон доходит до куратора и умирает на `codex: not found`.
- Задачу запускает `bin/hq-cron` — крошечный запускальщик, который только зовёт `hq wiki cron`. Вольт лежит под `~/Desktop`, это территория TCC, и доступ выдаётся конкретному пути. Отдельный запускальщик нужен, чтобы решение TCC не смешивалось с `bin/hq`, который запускается отовсюду и уже успел получить запрет. Собирается с `-trimpath` и без версии — байты не меняются от пересборки.
- `hq doctor` показывает `cron-pulse`: когда задача отработала и не отказал ли ей macOS в доступе к вольту. Проверить решения TCC: `sqlite3 ~/Library/Application\ Support/com.apple.TCC/TCC.db "select client, auth_value from access where service='kTCCServiceSystemPolicyAllFiles'"` — 2 значит разрешено, 0 запрещено.
- Одновременно идёт максимум один разбор: замок `~/.cache/wikipedik/drain.lock` с pid. Триггеров три, а куратор стоит десятки минут — иначе тихий вечер кончался бы тремя агентами в одном вольте.
- Если куратор не запустился вовсе (сломанный PATH, нет логина), штамп скоупа снимается: сбой окружения не должен блокировать репо на сутки.
- Дополнительный триггер — `SessionEnd` обоих профилей (`~/.claude`, `~/.codex` → `skills-stash/wiki/hooks/memory-drain-*.sh`): сессия закончилась, уроки записаны, самое время их разобрать.
- Если текущему репо разбор не нужен, хук делает свип: берёт скоуп с самым большим инбоксом. Один скоуп за раз — иначе завершение одной сессии стоило бы четырёх разборов.
- Шимы хуков заморожены и только зовут `hq hook session-start|session-end`: codex доверяет хуку по хэшу файла, и любая правка скрипта молча отзывает доверие. Правки логики идут в Go, файл хука не трогаем.
- `hq doctor` показывает пульс (`hook-pulse`): когда каждый хук срабатывал в последний раз. Проверять конфиг бесполезно — запись о доверии остаётся на месте, протухает только хэш.
- Гварды: не меньше 5 кандидатов и не чаще раза в 24 часа на скоуп (штамп в `~/.cache/wikipedik/`). Ставится ДО запуска куратора — упавший прогон не превращается в цикл повторов.
- Один прогон делает до 3 проходов и останавливается раньше, если проход не сдвинул счётчик: автономный куратор за раз разбирает далеко не весь бэклог.
- Каждое решение, запуск и пропуск, пишется в `~/.cache/wikipedik-autosync.log` и видно в `hq wikipedik`.

**Git sync policy:**
- `wiki sync` never commits by itself.
- `wiki sync --commit/--push` commits atomically by scope and uses company `git_email` from `.company-config` when present.
- WikiPedik commits go directly to WikiPedik `main`; app repo `feature → dev → main` delivery rules do not apply there.
- Commit message format remains platform-wide: English Conventional Commit type/scope, Russian description.
- Daily agents write only `_inbox.md`; curator commands own curated pages and git sync.


## aetheria
**Что:** Открыть воркспейс chimera/aetheria
**Запуск:** `aetheria`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/aetheria/`
**Repos:** Aetheria-App aetheria-frontend Aetheria-Manifests Aetheria-AI

## homeless
**Что:** Открыть воркспейс chimera/homeless
**Запуск:** `homeless`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/homeless/`
**Repos:** Jarvis Homeless Healler Hiring-Radar-Bot

## neuroslop4ik
**Что:** Открыть воркспейс chimera/neuroslop4ik
**Запуск:** `neuroslop4ik`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/neuroslop4ik/`
**Repos:** NeuroSlop4ik

## comonline
**Что:** Открыть воркспейс chimera/comonline
**Запуск:** `comonline`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/comonline/`
**Repos:** ComaOnline-Sources

## clients
**Что:** Открыть воркспейс SupportOps-Core/clients
**Запуск:** `clients`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/SupportOps-Core/clients/`
**Repos:** adminka

## cerebro
**Что:** Открыть воркспейс neurodesk/cerebro
**Запуск:** `cerebro`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/cerebro/`
**Repos:** cerebro

## <company> (neurodesk / chimera / SupportOps-Core)
**Что:** Вход с уровня компании: продукт → репо → worktree. Команда на каждую компанию с `.company-config` регистрируется автоматически.
**Запуск:** `neurodesk` (выбор продукта, затем репо) · `neurodesk agents` (выбор репо) · `neurodesk agents synapse` (сразу worktree)
**Иерархия:** компания (`neurodesk`) → продукт (`agents`) → продукт-шорткат сразу (`agents`).


## pizduk
**Что:** Открыть воркспейс chimera/pizduk
**Запуск:** `pizduk`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/pizduk/`
**Repos:** Pizduk
---

### Bootstrap

## new-company
**Что:** Создать компанию: workspace + SSH key + 1Password vault + AGENTS.md заготовка.
**Запуск:** `new-company <slug> <vcs[:host]> <namespace> [email]`
**См. также:** `new-project`

## new-project
**Что:** Создать продукт в компании: клонит репы + AGENTS.md заготовки + ярлык продукта + sync VS Code Project Manager.
**Запуск:** `new-project <company> <product> [--ns=<override>] [repo1 repo2 ...]`
**См. также:** `new-company`

---

### Agent context setup

## hq-skill
**Что:** Управление repo-owned skills: создать шаблон, установить symlinks в оба профиля (`~/.claude/skills`, `~/.codex/skills`), проверить frontmatter и links. Логика в ядре; `agent-skill` остался обёрткой.
**Запуск:**
- `hq skill list` — показать skills из `~/dotfiles/skills` и статус установки в обоих профилях
- `hq skill new <name>` — создать `~/dotfiles/skills/<name>/SKILL.md`
- `hq skill install` — поставить symlinks каждого skill в оба профиля
- `hq skill doctor` — проверить YAML frontmatter, symlinks и dangling-ссылки в профилях
**Используй:** когда добавляешь/меняешь reusable skill. Для agent-led изменений сначала используй `skill-maintainer`.

## dev-stack
**Что:** Единый локальный Docker-стек инфры для всех проектов. Сервисы: Postgres :5432, Redis :6379, MinIO :9000/9001, Qdrant :6333, ClickHouse :8123, Prometheus :9090, Grafana :3030, Loki :3100, Tempo :3200, OTel Collector :4317/4318, Langfuse :3001, Ollama :11434, Metabase :3002, CloudBeaver :8978 (+ Traefik/Homepage). Стек работает локально на этой машине; удалённые хосты вернутся вместе с серверным слоем (`hq server`). Телеметрия PUSH через OTLP (Prometheus сам не скрейпит). Источник правды: `services/dev-stack/`. Логика в ядре (`hq devstack`); zsh-обёртка держит только `export DEV_STACK_HOST`.
**Запуск:**
- `dev-stack connect [prod repo]` — самоонбординг проекта: блок в ./.envrc + БД `<product>__<repo>` + direnv allow
- `dev-stack doctor` — достижимость стека + подключён ли текущий проект
- `dev-stack up [svc...]` (всё или точечно) / `dev-stack down` / `dev-stack status` / `dev-stack pull`
- `dev-stack urls` — все адреса (учитывает DEV_STACK_HOST)
- `dev-stack psql [db]` / `dev-stack redis-cli` / `dev-stack clickhouse`
- `dev-stack envrc [prod repo]` (печать блока) / `dev-stack db-ensure <name>` / `dev-stack db-create <name>` / `dev-stack db-drop <name>` / `dev-stack logs [svc]` / `dev-stack nuke`
**Файлы:** `services/dev-stack/docker-compose.yml`, `services/dev-stack/config/`, `shell/60-devstack.zsh`.

---

### Secrets

## secret
**Что:** Секреты через 1Password — ЗАГЛУШКА. Старая реализация (скоупные item'ы + TTL-кэш) удалена на время редизайна; контракт будущего слоя (vault `Work-<co>`, имена `_company__<VAR>` / `<product>__<VAR>` / `<product>__<repo>__<VAR>`) описан в докстринге `core/secret`. Плейнтекст-секреты в git запрещены как и раньше.
**Запуск:**
- `secret signin` — login в 1Password (работает как раньше)
- всё остальное отвечает «не реализовано» до нового слоя

---

### Direnv

## direnv
**Что:** Авто-загрузка env vars из `.envrc` при `cd`.
**Запуск:** `direnv allow` (один раз на папку), `direnv reload`, `direnv deny`

---

### Setup

## transcript-scrub
**Что:** Маскирует секреты в JSONL-транскриптах сессий (`~/.codex/sessions`, `~/.claude/projects`) и дайджестах вольта. Маскирует, не удаляет: оставляет узнаваемый префикс (`ghp_***MASKED***`), чтобы было видно, какой ключ утёк. Ловит github/gitlab/anthropic/openai/openrouter/grafana-sa/langsmith/aws/slack/npm/hf/stripe/google/sendgrid/telegram/jwt + bearer-токены, пароли в conn-string, `KEY=value`/`"api_key": "..."` присвоения; плейсхолдеры (`${VAR}`, `xxxx`, `REPLACE_ME`) пропускает. Сохраняет mtime (не ломает ingest-манифест), пропускает файлы моложе 24ч (живые сессии), JSON остаётся валидным, идемпотентен.
**Запуск:**
- автоматически — launchd-джоба `com.gerc0g.transcript-scrub` ежедневно в 03:30, лог `~/Library/Logs/transcript-scrub.log`.
- вручную — `python3 ~/dotfiles/scripts/transcript-scrub.py [--dry-run] [--min-age-hours N]`.
**Файлы:** `scripts/transcript-scrub.py`, `scripts/launchd/com.gerc0g.transcript-scrub.plist`.

## bootstrap.sh
**Что:** Полная установка platform на свежем Mac (brew packages + codex/claude + симлинки + профили). Один раз.
**Запуск:** `~/dotfiles/bootstrap.sh`

---

### Troubleshooting

## op-not-signed-in
**Что:** Если `op` CLI не подключен → Settings → Developer → Integrate with CLI ON, потом `secret signin`.

## tmux-prefix-broken
**Что:** Если Ctrl-b не работает в tmux (Ghostty + русская раскладка). Workaround: English layout перед prefix.

## direnv-not-loading
**Что:** Если `.envrc` не подгружается → проверь `direnv allow`, hook в shell, ошибки.

## ssh-permission-denied
**Что:** Если git clone не работает → проверь публичный ключ в GitLab/GitHub Settings → SSH Keys.
