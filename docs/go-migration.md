# Переезд платформы на Go-ядро (hq)

Рабочий трекер финальной миграции bash/zsh/python → `core/`. Цель: вся
доменная логика живёт в `hq`, снаружи остаются только тонкие zsh-обёртки
(cd, export, alias), direnv/op-хуки и fail-safe шимы.

Статус на старте: в Go — модель мира (`hq ls`), `setup`/`doctor`,
`workspace`. Вне ядра ~3.5 тыс. строк живой логики. Модель мира
продублирована 5 раз вне ядра; формат `.agent-workspace` пишут 4 кодовых
пути; ядро форкает python (`core/workspace/create.go` →
`vscode-projects-sync.py`).

## Этап 0 — зачистка мёртвого кода (до порта)

- [ ] `scripts/new-project.sh:102-119` — генерирует лаунчер на удалённый
      `launch` и ломает табличный `shell/30-projects.zsh`; убрать генерацию
      функций и вставку в `COMMANDS.md`, печатать подсказку про
      `_PROJECT_SHORTCUTS`.
- [ ] `skills-stash/wiki/scripts/install-wiki-runtime.sh` — dangling-симлинки
      на несуществующий `worker/`, дословные дубли строк (47-48, 59-60),
      двойной `ensure_codex_hook` (165-166).
- [ ] `README.md` — артефакты замены «5 профилей → 2»: тройной
      `codex login` (26-28), пять одинаковых буллетов (50-54), дубли в
      Validation (372-382).
- [ ] Ссылки на мёртвый `launch`: `docs/COMMANDS.md`, `docs/BACKLOG.md`,
      `docs/wikipedik-roadmap.md`, `docs/wikipedik-cheatsheet.md`,
      `scripts/wikipedik-autocommit.sh:10,15`, `shell/71-wiki.zsh:372`,
      `core/world/world.go:6`.
- [ ] `Makefile` — `bash -n` по zsh-файлу `wiki-bootstrap-product.sh`
      (нужен `zsh -n`), пустой глоб `skills-stash/wiki/scripts/*.py`.
- [ ] `skills-stash/wiki/README.md` — несуществующий
      `fetch-wiki-skills.sh`, описание уехавшего `worker/`.
- [ ] `docs/COMMANDS.md:190-193` — старые профили daily/setup/wiki в
      описании `agent-skill`.
- [ ] Мусор в git: `scripts/__pycache__/*.pyc`,
      `tools/status-tui/status-tui`, `.agents/oracle/requests/*` —
      `git rm --cached` (в `.gitignore` уже есть).

## Этап 1 — фундамент: контекст и editor-sync

- [ ] `hq ctx` — единый резолвер company/product/repo(/worktree) из `$PWD`,
      включая разворот `.worktrees/...` обратно в репо; plain-вывод для
      скриптов. Источник истины — `core/world` + `core/workspace`.
- [ ] `hq editor sync` — порт `scripts/vscode-projects-sync.py` (382);
      `syncEditorProjects` в `create.go` становится вызовом функции.
      Обновить `shell/33-vscode-projects.zsh`, `scripts/new-project.sh`.
- [ ] `hq workspace path <co> <prod> <repo> <id>` — убрать знание раскладки
      из `shell/32-agent-workspace.zsh`.

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
