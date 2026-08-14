# Переезд платформы на Go-ядро (hq)

Рабочий трекер финальной миграции bash/zsh/python → `core/`. Цель: вся
доменная логика живёт в `hq`, снаружи остаются только тонкие zsh-обёртки
(cd, export, alias), direnv/op-хуки и fail-safe шимы.

Статус на старте: в Go — модель мира (`hq ls`), `setup`/`doctor`,
`workspace`. Вне ядра ~3.5 тыс. строк живой логики. Модель мира
продублирована 5 раз вне ядра; формат `.agent-workspace` пишут 4 кодовых
пути; ядро форкает python (`core/workspace/create.go` →
`vscode-projects-sync.py`).

## Этап 0 — зачистка мёртвого кода (до порта) — DONE

- [x] `scripts/new-project.sh` — убрана генерация мёртвых launch-лаунчеров
      и вставка в `COMMANDS.md`; вместо этого подсказка про
      `_PROJECT_SHORTCUTS`.
- [x] `install-wiki-runtime.sh` — убраны dangling worker-симлинки и дубли.
- [x] `README.md` — артефакты «5 профилей → 2» + мёртвая команда `agent`.
- [x] Ссылки на мёртвый `launch` вычищены (COMMANDS, BACKLOG, roadmap,
      cheatsheet, autocommit, 71-wiki). `core/world/world.go:6` оставлен —
      это корректная историческая мотивация пакета.
- [x] `Makefile` — shell-check выбирает bash/zsh по шебангу. Пустой глоб
      `*.py` в python-check оставлен (безвреден, future-proof).
- [x] `skills-stash/wiki/README.md` переписан под текущий состав стеша.
- [x] `docs/COMMANDS.md` — agent-skill описан без daily/setup/wiki.
- [x] Снято: «мусор в git» — `__pycache__`/status-tui/.agents на самом деле
      не закоммичены (untracked под gitignore), чистить нечего.

## Этап 1 — фундамент: контекст и editor-sync — DONE

- [x] `hq ctx [--plain]` — `workspace.Locate` (+ `world.Is*Dir` хелперы),
      резолвит company/product/repo/worktree с любой глубины.
- [x] `hq editor sync` — пакет `core/editor` (merge-логика + тесты),
      `workspace.SyncEditorProjects` — вызов функции вместо форка python;
      `vscode-projects-sync.py` удалён, обёртка сохраняет старое имя.
      Бонус: маркерная модель вычистила мусорные записи (rca-synapse) из
      Project Manager.
- [x] `hq workspace path` — `shell/32` больше не знает раскладку пула.

## Этап 2 — git-обвязка агентов — DONE

- [x] `hq commit` — порт agent-commit.sh; identity-check теперь работает и
      в worktree (bash его там молча пропускал).
- [x] `hq finish [--no-review]` — порт agent-finish + task-push; метаданные
      finish (pushed_branch, review_base, review_opened_at,
      ready_without_review_at) добавлены в `core/workspace` — раньше Go
      молча терял эти ключи при перезаписи.
- [x] `hq skill {list,new,install,doctor}` — порт agent-skill.sh;
      `make lint` (skill-check: build → `hq skill doctor`) и `bootstrap.sh`
      идут через ядро.
- [x] agent-commit/finish/task-push/skill — 3-строчные шимы; BASELINE,
      PLATFORM, COMMANDS, git-workflow переведены на `hq ...`.

## Этап 3 — secrets — DONE

- [x] `hq secret {add,edit,name,envline,list,get,cache}` — пакет
      `core/secret`; контекст через `workspace.Locate` + slug-оверрайды;
      кэш бинарно совместим (sha256-ключи, та же раскладка файлов).
- [x] `scripts/secret-cache.sh` → shim (путь вморожен в `.envrc`).
      В zsh остался только `secret signin` + диспетчер.
- [x] Гоча: бланкетный `*secret*` в `.gitignore` молча выкинул бы
      исходники ядра (как когда-то secret-cache.sh, фикс a79fb4a) —
      добавлены исключения `!core/secret/`.
- [ ] Единый писатель `.envrc` — закрыть на этапе 4 (devstack).

## Этап 4 — dev-stack — DONE

- [x] `hq devstack {up,down,restart,pull,logs,ps,psql,redis-cli,ch,urls,
      envrc,connect,doctor,db-ensure,db-create,db-drop,nuke}` — пакет
      `core/devstack`; probes через `net.DialTimeout`, ctx через
      `workspace.Locate`, интерактивные клиенты — сквозной stdio.
      В zsh остался `export DEV_STACK_HOST` + диспетчер. Оба писателя
      `.envrc` (secret, devstack) теперь в ядре.

## Этап 5 — wiki — DONE

- [x] `hq wiki {status,sync,synthesize,commit}` — пакет `core/wiki`:
      scope-резолвер, порcelain-парсер, preflight/postflight, коммит с
      per-company автором, запуск куратора (codex, TTY-детект). Хардкод
      `~/Desktop/WikiPedik` заменён на `WIKIPEDIK_ROOT` везде.
- [x] `hq wiki bootstrap` — шаблоны как константы + render, privacy и
      идемпотентность под тестами; sed-регистрация индексов → Go.
- [x] `hq wiki hot-refresh` + `hq wiki rules-sync` — общий парсер правил
      (`paths:` frontmatter); чекауты из `world.Scan` (мусорные каталоги
      больше не получают линков). Смоук нашёл 12 недолинкованных правил
      в worktree. Хук в `workspace start` — на этапе 6.
- [x] `hq wiki autocommit [--if-due]` — secret-scan + best-effort
      pull/push; вопрос планировщика (TCC) остаётся открытым (BACKLOG).
- [x] `agent-session-digest.py` остаётся python осознанно: парсит внешние
      JSONL-форматы, модель мира не дублирует, вызывается куратором.
      Объединение секрет-паттернов — вместе с возможным портом
      transcript-scrub (этап 6+).

## Этап 6 — полировка поверхности — DONE

- [x] rules-sync вызывается при `hq workspace start` — свежий worktree
      сразу получает binding-правила.
- [x] `hq hook session-start --agent claude|codex` — тела хуков в ядре
      (без python3); шимы гарантируют `exit 0`. Хуки уважают
      `PROKECTFILES_ROOT`/`WIKIPEDIK_ROOT`.
- [x] status-tui собирается в `bin/` через `make build`; цепочка
      zsh→bash→`go run` убрана. Слияние в `hq status` отложено осознанно:
      TUI живёт отдельным модулем (см. память status-tui direction).
- [x] `hq commands [имя]` — рендер `docs/COMMANDS.md` вместо awk;
      `?`/`help` — обёртки.
- [x] `hq onboard company|product` — порт new-company/new-project;
      envsubst-шаблоны рендерятся `os.Expand` (файлы шаблонов не
      менялись); у `hq setup` отобран алиас `onboard`.
- [x] install-wiki-runtime.sh + python-блок bootstrap.sh → 4 шага
      `core/setup` (session-hooks, curator-skills, claude-settings,
      codex-config). `hq doctor` тут же поймал реальный дрейф: curator-
      скиллы не были прилинкованы после рефакторинга «5 профилей → 2».

## Итог миграции (2026-08-14)

Вся доменная логика платформы — в `core/` (~11 тыс. строк Go с тестами).
Вне ядра осознанно остаются:
- тонкий zsh-слой: cd/редактор, export'ы, `eval` (op signin, direnv),
  alias `?`, динамические функции компаний, таблица `_PROJECT_SHORTCUTS`;
- шимы обратной совместимости (agent-*, secret-cache, wiki-*, хуки);
- `transcript-scrub.py` + launchd (автономный ночной джоб на системном
  python) и `agent-session-digest.py` (парсер внешних JSONL-форматов);
- `tools/status-tui` — отдельный Go-модуль, собирается в `bin/`.
Единственная заглушка ядра — `hq server` (новая функциональность, не порт).
Домен заглушки вынесен в пакет `core/server` (Registry/Server/
ErrNotImplemented): CLI не держит доменной логики даже для заглушки, полная
multi-server реализация приземлится в готовое место.

## Вне переезда (осознанно)

- Тонкий zsh-слой: cd/редактор, export'ы, `eval "$(op signin)"`,
  `eval "$(direnv hook zsh)"`, alias `?`, динамические функции компаний.
- `transcript-scrub.py` + launchd-plist — автономный ночной джоб на
  системном python; порт только вместе с проверкой «джоб реально
  отработал» в `hq doctor`.

## Известные дыры (бонусы миграции)

- Три разных списка секрет-паттернов: `transcript-scrub.py` (22),
  `agent-session-digest.py` (6), `wikipedik-autocommit.sh` (6).
- Дайджесты в вольте не дочищаются ночным скраббером (TCC на Desktop).
- `agent-debug` в `05-agent-defaults.zsh` дублирует нишу `hq doctor`.
