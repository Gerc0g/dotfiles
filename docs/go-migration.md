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

## Этап 2 — git-обвязка агентов

- [ ] `hq commit "type(scope): описание" -- <paths>` — порт
      `agent-commit.sh` (132); `.company-config` читает `world.Config`.
- [ ] `hq finish [--no-review]` — порт `agent-finish.sh` (178) +
      `agent-task-push.sh` (82, растворяется во флаге); единственный
      писатель `.agent-workspace` — `core/workspace`.
- [ ] `hq skill {list,new,install,doctor}` — порт `agent-skill.sh` (215);
      `make lint` и `bootstrap.sh` переходят на `hq skill`;
      `skillLinksStep` в `core/setup` перестаёт дублировать doctor.
- [ ] Скрипты остаются 3-строчными шимами (конвенции в BASELINE/PLATFORM
      обновить на `hq ...`).

## Этап 3 — secrets

- [ ] `hq secret {add,edit,cache}` — порт `shell/50-secrets.zsh` (342,
      кроме `secret signin`) и `scripts/secret-cache.sh` (154).
- [ ] `scripts/secret-cache.sh` → shim `exec hq secret cache "$@"` —
      путь вморожен в `.envrc` проектов, не переписываем их.
- [ ] Единая логика записи `.envrc` в ядре (сейчас два писателя:
      secrets и devstack).

## Этап 4 — dev-stack

- [ ] `hq devstack {up,down,restart,pull,logs,ps,psql,redis-cli,ch,urls,
      envrc,connect,doctor,db-ensure,db-create,db-drop,nuke}` — порт
      `shell/60-devstack.zsh` (253); `nc` → `net.DialTimeout`; ctx из
      `hq ctx`. В zsh остаётся `export DEV_STACK_HOST` + тонкая функция.

## Этап 5 — wiki

- [ ] `hq wiki {status,sync,synthesize,commit}` — порт `shell/71-wiki.zsh`
      (375): scope-резолвер, парсер porcelain, preflight/postflight,
      коммит с per-company автором. Починить хардкод `~/Desktop/WikiPedik`
      (игнорирует `WIKIPEDIK_ROOT`).
- [ ] `hq wiki bootstrap` — порт `wiki-bootstrap-product.sh` (712);
      шаблоны → `embed.FS`; privacy-firewall с тестом.
- [ ] `hq wiki hot-refresh` + `hq wiki rules-sync` — порт двух py
      (164+172) с общим парсером правил (`paths:` frontmatter);
      rules-sync вешается в хук `hq workspace start`.
- [ ] `hq wiki autocommit` — порт `wikipedik-autocommit.sh` (69);
      вопрос планировщика (TCC) — отдельно.
- [ ] `hq wiki digest` — порт `agent-session-digest.py` (262), низкий
      приоритет; общий список секрет-паттернов с transcript-scrub.

## Этап 6 — полировка поверхности

- [ ] `hq status` — убрать цепочку zsh→bash→`go run`, status-tui под ядро.
- [ ] `hq help` — рендер `docs/COMMANDS.md` вместо awk в `99-help.zsh`.
- [ ] `hq onboard company|product` — порт `new-company.sh` (152) +
      `new-project.sh` (176); envsubst → `text/template`; `~/.ssh/config`
      как шаг `core/setup`; шорткаты — данные, не код в zsh.
- [ ] SessionStart-хуки → шимы `exec hq hook session-start --agent=...`
      с `|| exit 0` (fail-safe контракт сохраняется).
- [ ] `install-wiki-runtime.sh` → шаги `core/setup` (клон вольта,
      curator-симлинки, codex `config.toml`).

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
