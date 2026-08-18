# Platform Backlog

## Домашний Ubuntu-ноут как dev-сервер (поэтапно)

Status: исследование / поэтапно. Ноут (16 ГБ, Ubuntu, дома 24/7, в розетке)
как персональный dev-сервер. Мак = рабочая машина, телефон = подрулить.
Доступ — Tailscale (приватная сеть, ноль открытых портов).

Фундамент (принято): нельзя «синкать/зеркалить» живые tmux-сессии между
машинами — это live process migration Mac↔Linux, физически невозможно.
Модель только одна: сессия живёт на ОДНОЙ машине, остальные `tmux attach`
(один живой экран, много клиентов). Веб-интерфейсы приложений ноута
открываются на маке/телефоне по tailnet-адресу (`http://ноут:3000`) — Tailscale.

Этапы (от безопасного к рискованному):
1. **Инфра на ноут** (безболезненно): только `dev-stack` уезжает на ноут +
   Tailscale. Агенты/код/WikiPedik остаются на маке. Проекты ходят к инфре
   ноута через `DEV_STACK_HOST=<tailnet>`. Уже server-ready заложено.
   - launchpad: скрипт деплоя compose на ноут (rsync services/dev-stack + up).
   - мак не меняется. Тест: проект с мака видит Postgres/Qdrant/… ноута.
2. **Агенты на ноут** (эксперимент): воркспейс поднимается на ноуте по ssh (`hq workspace` + tmux),
   мак/телефон аттачатся. Даёт телефон-доступ (Blink + Tailscale → tmux attach).
   - РИСК: WikiPedik вольт локален на маке. Агенты на ноуте пишут в вольт
     ноута. Решение: вольт = git-репо → синк push/pull (автокоммит уже есть),
     либо Syncthing/Obsidian Sync. Тестировать на ОДНОМ репо, не ломая флоу.
   - Код проектов тоже переезжает на ноут (Клод правит файлы там, где запущен).
   - Открытый вопрос: офлайн-работа (без сети до ноута работы нет) — нужен ли
     гибрид «часть локально на маке».
3. **Телефон**: Blink Shell (iOS) + Tailscale app → ssh ноут → tmux attach.
   Панель отображается 1:1 (тот же tmux). `prefix+z` zoom панели на мелком экране.


## WikiPedik: working memory + salvage артефактов worktree

Status: done (2026-06-11) — `salvage_worktree_artifacts` + `drop_salvaged_junk`
в `agent-workspace.sh` (вызываются из `remove` и `cleanup`); Phase 0 в
`inbox-drain` (куратор разбирает `_salvage/` при `wiki sync`); конвенция
`docs/epics/<task>.md` в PLATFORM.md; e2e-тест пройден. Следующие шаги серии
(2 backfill, 3 урок→правило) — ниже по записям.

Problem: знание двух классов теряется безвозвратно. (1) Working memory —
состояние задач между сессиями (epic-доки, планы, "на чём остановился")
умирает с worktree или блокирует его удаление (кейс: synapse/5f256d2b держим
из-за 3 несохранённых epic-доков). (2) Oracle-ответы (second-model review)
живут в `.agents/oracle/` и удаляются при cleanup как мусор.

Design (продумано 2026-06-11):
- `salvage_worktree_artifacts` в `agent-workspace.sh`: перед `git worktree
  remove` (в `remove` и `cleanup`) копировать ценные несохранённые артефакты
  в вольт: `20-projects/<co>/<prod>/repos/<repo>/_salvage/<worktree-id>/`.
  Что брать: `.agents/oracle/*.md`, untracked/modified `docs/epics/*.md`.
  Плюс `INFO.md` с метаданными (branch, task, дата, источник).
- После salvage: `.agents/` удалять (runtime-мусор, ответы спасены),
  untracked epic-доки удалять (копия в вольте) — тогда завершённый worktree
  удаляется без --force. Изменённые tracked-файлы (реальный код) по-прежнему
  блокируют remove.
- Куратор: `_salvage/` разбирается при `wiki sync` как второй inbox
  (промоутить durable, мусор удалить) — дополнить `inbox-drain` SKILL.md.
- Конвенция в PLATFORM.md: состояние задачи живёт в `docs/epics/<task>.md`
  и коммитится с веткой (тогда оно не теряется вообще).
- Privacy: salvage по построению остаётся в company namespace.

Acceptance criteria:
- `agent-workspace remove/cleanup` не уничтожает oracle-ответы и epic-доки.
- Worktree с «только epic-доками» удаляется без ручного rm.
- `_salvage/` упоминается в curator-флоу; COMMANDS.md обновлён.

Порядок серии: (1) этот salvage → (2) backfill базы через
`agent-history-ingest` по июньским сессиям (см. wikipedik-roadmap.md §2) →
(3) promotion "урок → path-scoped правило" (см. memory v2 §1, делать после
наполнения базы — правилам нужны повторяющиеся уроки).

## WikiPedik: использовать полную мощь подхода

Status: todo

Цикл памяти работает на страховочном минимуме; неиспользованные возможности и
рекомендованный порядок внедрения — в [wikipedik-roadmap.md](./wikipedik-roadmap.md).
Топ-3: history-ingest backfill июня, bootstrap самого dotfiles в память,
wiki-context-pack в workspace-флоу по умолчанию.

## WikiPedik memory v2 — по итогам research рынка и SOTA

Status: mostly done 2026-06-12.
- §1 done: "урок → правило" (`wiki-rules-sync.py`, дайджест Binding rules в
  hot.md, promotion bar в wiki-synthesize) + hot.md как индекс (футер memory
  pages, read-on-demand протокол в чекпойнте хуков).
- §2 done: citations обязательны в lesson-append, verify-before-apply в
  чекпойнте обоих хуков.
- §3 done: memory evolution (Zettelkasten-связки + refresh Last verified)
  как шаг Phase 4 в inbox-drain.
- §4 todo: last-used экспирация — ждёт первой практики wiki lint.
- §5 blocked: autoMemoryDirectory vs privacy firewall (открытый вопрос
  research).
Бонус: сам dotfiles подключён к памяти (_platform scope в вольте, симлинки,
хуки пускают ~/dotfiles), 5 платформенных уроков в inbox.

Источник: [wikipedik-memory-research.md](./wikipedik-memory-research.md)
(deep-research 2026-06-11, 23 источника, верифицированные claims). Рынок
(Claude Code auto memory, VS Code/Copilot, Anthropic memory tool) сошёлся на
нашей архитектуре — фундамент не менять, докрутить механики:

1. **hot.md → индекс ~150-200 строк** (рыночный бюджет 200 строк/25KB) с
   однострочными указателями на topic-страницы; в SessionStart-чекпойнт —
   протокол "view memory dir first, read on demand" (Anthropic memory tool).
   Path-scoped `.claude/rules/` + симлинки как selective deep read по
   подсистемам (учесть баги #21858, #23478, #23569).
   **Двухканальная доставка правил (оба агента):** правило живёт в вольте
   (`repos/<repo>/rules/*.md`) один раз; (a) Claude — симлинк в
   `.claude/rules/` с `paths:` (полный текст, лениво при касании файлов);
   (b) codex — секция "Binding rules" в hot.md: однострочный дайджест всех
   правил репо, собирается `wiki-hot-refresh` детерминированно, бюджет
   5-10 строк. Роли совпадают с каналами: плану — ограничения на старте,
   коду — детали в момент правки. Исследовать: могут ли codex-хуки
   (не только SessionStart) инжектить контекст по событию чтения/правки
   файла — тогда path-scoped появится и у codex.
2. **JIT-верификация цитат (паттерн GitHub Copilot)**: обязательное поле
   `citations: file:line/commit` в схеме `lesson-append`; перед применением
   урока worker проверяет цитаты в коде, при противоречии пишет исправленную
   версию в inbox. Заменяет offline-курацию противоречий.
3. **Zettelkasten-линки + эволюция памяти (A-MEM, NeurIPS 2025)**: frontmatter
   wikilinks между curated-страницами; при drain нового урока куратор
   обновляет связанные старые страницы; multi-hop поиск обходом графа.
4. **Last-used экспирация**: трекинг использования curated-записей, кандидаты
   в архив через `wiki lint` (мягкий вариант 28-дневной экспирации Copilot).
5. **`autoMemoryDirectory` Claude Code → staging-зона вольта** под куратора;
   блокер: развести per-git-repo keying с privacy firewall по компаниям.

Не делать: vector DB/embeddings (ценность в графе связей, не в векторах),
hosted-платформы как замена (lock-in), авто-майнинг сессий без approve
(Cursor удалил Memories в 2.1.x именно поэтому — curator-in-the-loop).

## Automatic WikiPedik vault commits

Status: done (2026-06-11) — `scripts/wikipedik-autocommit.sh`: catch-all commit с secret-scan, без push; вручную — `wiki autocommit`. launchd отклонён из-за TCC-доступа к Desktop у фоновых процессов. UPDATE 2026-08-14: суточный вызыватель (`launch --if-due`) удалён вместе с tmux-раскладкой — автокоммит снова только ручной, нужен новый планировщик.

Problem: nothing commits the WikiPedik vault automatically — no obsidian-git plugin, no launchd/cron job, no hook. The vault has 8 manual commits total; curator work (drain, synthesize, hot.md refresh) sits uncommitted until someone runs `wiki-commit` by hand. Human edits in Obsidian are never captured at all.

Goal: vault changes get committed regularly without manual `wiki-commit`, while keeping the existing scope-atomic curator commits meaningful.

Acceptance criteria:
- A scheduled job (launchd) or `wiki sync` tail commits vault changes at least daily.
- Curator-scope changes keep the scope-atomic convention (`docs(wiki): синхронизировать память <scope>`, company `git_email` from `.company-config`); a periodic catch-all commit (e.g. `chore(vault): автокоммит несинхронизированных изменений`) covers the rest (`.obsidian/`, root files, human edits).
- Auto-commit never pushes — push stays explicit (`wiki sync --push` or manual `git push`).
- Secrets/privacy lint runs (or is at least planned) before catch-all commits so junk and sensitive content do not get silently enshrined in history.
- Decide interaction with `_wiki_dirty_outside_scope` guard: auto-commits must not break `wiki sync --commit` scope refusal logic.

## Stale active worktree handling

Status: done (2026-06-11) — `agent-workspace stale [days]` показывает active worktrees без tmux-сессии и коммитов N+ дней (dirty/unpushed счётчики); `start` печатает подсказку при наличии stale. Политика веток: `remove`/`cleanup` удаляют ветку worktree, если она merged/pushed; `agent-workspace prune-branches` чистит остальные; ветки, привязанные к worktree или с локальной работой, не трогаются никогда.

Problem: `agent-workspace start` already runs `cleanup_workspaces --days 7 --quiet`, but cleanup only removes `ready` (or merged `review`) worktrees. In practice tasks are rarely finished through `agent-finish.sh`, so worktrees stay `cleanup_state=active` forever and accumulate (2026-06-10: 20 worktrees, 4 live tmux sessions, all but one `active`).

Goal: stale `active` worktrees become visible and easy to triage instead of accumulating silently.

Acceptance criteria:
- Stale `active` worktrees (no tmux session, no commits for N days) are surfaced — e.g. a warning in `launch` output or an `agent-workspace list --stale` view with dirty/ahead status.
- Cleanup still never touches dirty or unpushed work; triage of stale worktrees stays a human decision (push / mark ready / remove).
- Decide policy for leftover `agent/work-*` branches after worktree removal (today branches accumulate with no cleanup path).

## Bootstrap-managed agent assets

Status: todo

Problem: skills, agent configs, TUI tools, and local infrastructure are currently a mix of repo files plus manual copies into `~/.codex`, `~/.claude`, and other home directories.

Goal: `~/dotfiles/bootstrap.sh` should make a fresh Mac or fresh agent profile reproducible from this repo.

Acceptance criteria:
- Bootstrap installs/syncs repo-owned skills into Codex and Claude skill directories.
- Bootstrap installs/syncs agent configs and profile files from dotfiles.
- Bootstrap builds or links local TUI tools (status-tui).
- Bootstrap verifies required CLIs: `codex`, `claude`, `oracle`, `tmux`, `direnv`, `op`, `go`.
- Bootstrap prints manual steps only for things that cannot be automated safely, such as browser login or 1Password approval.
- Re-running bootstrap is idempotent and does not overwrite user-local secrets.


## Aider-style: коммит на каждую правку (для dotfiles/агентов)

Status: сделано для вольта (2026-08-15), для кодовых репо — backlog

Реализовано в зоне research: PostToolUse-хук профиля коммитит вольт после
каждого шага агента (локально, без push), Stop/SessionEnd и выход из
`wikipedik` добирают остаток и пушат. Сообщение генерится из porcelain-статуса
(«создан X», «дополнен Y», «удалён Z») — без LLM. Реализация: `hq wikipedik
autosync [--push]`, `core/wiki.AutoMessage`.

Осталось для кодовых репо: там коммит должен проходить `make verify` и явные
пути (`hq commit`), поэтому механика вольта в лоб не переносится.

Исходная идея (2026-06-22):

Idea: перенять у Aider подход «каждое логическое изменение = отдельный git-коммит»
— строго, автоматически, с осмысленным сообщением по диффу. Полезно: прозрачная
история, лёгкий откат, аудит «что агент реально сделал». Сейчас у нас есть
`AGENT_GIT_MODE=commit-local` + `scripts/agent-commit.sh`, но без авто-коммита на
каждое изменение и без авто-генерации сообщения.

Что изучить у Aider (прочитать код, заимствовать подход — НЕ зависимость):
- как генерит commit-message из диффа;
- когда именно коммитит (после каждой правки/файла), гранулярность;
- `/undo` (откат последнего AI-коммита) и `--no-auto-commits` как опция;
- работа прямо на git-дереве без песочницы.
Repo: https://github.com/Aider-AI/aider · https://aider.chat/docs/git.html

Goal: усилить `agent-commit.sh` / `commit-local` режим авто-коммитом-на-изменение
с авто-сообщением, опциональным (флаг), с лёгким откатом. Сначала прочитать код
Aider, затем интегрировать в наш флоу.

## Гибрид Mac+сервер: агенты, инфра, синк (модель закреплена; детали — позже)

Status: decided (модель) + research/todo (детали), 2026-06-23

ЗАКРЕПЛЕНО (решение):
- Гибрид: часть агентских сессий на Mac, часть на сервере gerc0g. Сервер слабый —
  всю обвязку вынесем на мощный сервак позже; это промежуточный этап.
- Worktree: на каждой машине свои (своя копия репо, своя ветка, уникальный id);
  встречаются только в облаке (git remote). Flow дотфайлов НЕ ломается (git-native).
- Инфра: ОДНА dev-stack, строго на сервере. Правило по контексту:
  - агенты НА сервере → DEV_STACK_HOST=localhost
  - агенты на Mac / др. устройствах → DEV_STACK_HOST=gerc0g (по tailnet)
  Уже поддержано переменной `${DEV_STACK_HOST}`; нужно лишь `export DEV_STACK_HOST=localhost`
  на сервере. `.envrc` проектов одинаков на обеих машинах — резолвится по-разному.
- WikiPedik синкается через GIT push/pull, НЕ Mutagen/Syncthing. Mutagen — только для
  «правлю код локально, исполняю на сервере», у нас не используется.

НА ПОТОМ (разобрать/заресёрчить + внедрить):
- Полный git-sync флоу: правила синхронизации по репозиториям и внутри проектов.
- WikiPedik: автокоммит + PUSH (сейчас never-push, scripts/wikipedik-autocommit.sh) +
  PULL на старте (cron / shell-hook). Политика конфликтов (append в _inbox.md обычно тривиален).
- Опц.: имя БД с id worktree (`<product>__<repo>__<id>`), чтобы параллельные фичи на
  Mac и сервере не делили данные общей dev-stack.
- Кроны на pull (вольт/доки): частота, где вешать (launchd на Mac / systemd на сервере).
Файлы-якоря: scripts/wikipedik-autocommit.sh, shell/60-devstack.zsh,
docs/platform/local-infra.md, agent-profiles/BASELINE.md.


## Research-зона WikiPedik: редизайн под обучение (2026-08-14)

Status: DONE 2026-08-17 — зона построена: изолированный профиль codex,
контракт AGENTS.md, шаблон конспекта, линтер, index/map-candidates, граф
Obsidian, автокоммиты агентом. Конспектов пока ноль — механика ждёт материала.

Старые зоны `research` и `Personal Brand` снесены под чистую (история в вольте,
коммит `96355d9`). Каркас `hq wikipedik` (зоны, path, sync, обзор) готов.

Согласовано по research:
- Модель **topic-centric**, не source-centric: единица — тема, которую пользователь
  изучил сам; агент помогает конспектировать, объяснять и повторять. Автоматического
  ingest источников НЕТ (на нём надорвалась старая зона).
- Одна тема = один растущий файл. Дописывается внутрь секций, не в конец.
- Структура: `topics/<домен>/` (плоско), `maps/` (карты тем вместо папок-подтем),
  `sources/` (паспорта), `raw/` (оригиналы, неприкосновенны, + `raw/inbox/`),
  `AGENTS.md`, `index.md` (генерируется), `log.md`.
- Формат конспекта — «сэндвич»: обязательные шапка (frontmatter + H1 + «В двух
  словах») и хвост («Вопросы для себя», «Связи», «Источники»), свободная середина
  под тип материала. Формулы обязательно в LaTeX.
- Всё производное вычисляется, не ведётся руками (index, очередь повторения, линт).
- Интерфейс пользователя — ТОЛЬКО чат с агентом; `hq research …` — инструмент агента
  с машинным выводом, не пользовательская команда.
- Агент зоны — **codex**.

Открытые вопросы (решить до/во время реализации):
- Имена файлов: англ. kebab-case + русские `aliases` или сразу русские.
- Стартовые домены (3-5).
- Механика повторения: `last_reviewed` + «давно не повторял» или интервалы (SM-2).

Отложено на после реализации (явная договорённость):
- **План коммитов вольта**: Obsidian Sync (подписка есть, синкает файлы между
  устройствами) vs git-история/автокоммит. Кто когда коммитит, риск конфликтов
  Sync↔автокоммит, нужен ли push и планировщик (см. «Automatic WikiPedik vault
  commits» и TCC-гочу).
- **Правила `AGENTS.md`** зоны: финальная редакция контракта ментора (фазы работы,
  дисциплина связей и цитат, статусы зрелости, обязательные вызовы инструментов).
- Мобильное повторение: сейчас закрывается Obsidian Sync (чтение); нужен ли
  отдельный канал записи с телефона — позже.


## Routines: единая сущность управления фоновыми джобами (2026-08-16)

Status: DONE 2026-08-18 — `core/routine`, `hq routine`, реестр + launchd-генерация.
Сделано: реестр (встроенные + пользовательские из `~/.config/hq/routines.yaml`),
list/run/logs/enable/disable, гварды (интервал + суточный потолок, у агентских
обязателен), единый журнал `~/.cache/hq/routines.log` (туда же ушёл
`wikipedik-autosync.log`), шаги `routines` и `routine-pulse` в `hq doctor`,
плисты генерируются из реестра, старые рукописные сняты.
Планировщик: launchd. Открытый вопрос TCC закрыт — доступ выдаётся по пути, и
задачи ходят через неизменяемый `bin/hq-cron`, у которого свой грант.
Не сделано: счётчика реально потраченных токенов нет — измерить их снаружи
нечем, ограничение выражено числом запусков за сутки.

Problem: фоновая работа расползлась по разным механизмам и её никто не видит
целиком. Сейчас это: launchd-джоба `transcript-scrub`, Stop/SessionEnd-хуки
зоны research (автокоммит вольта), автодрейн памяти (`hq wiki drain`),
`--if-due`-гварды в нескольких местах, `wikipedik autosync` при выходе из зоны.
Часть из них запускает АГЕНТА, то есть жжёт токены подписки — и забытая джоба
тратит деньги молча. Классический риск «поставил крон и забыл» здесь дороже
обычного.

Goal: `hq routine` — отдельная сущность платформы: реестр фоновых задач с
мониторингом, управлением и видимой стоимостью.

Acceptance criteria:
- реестр: `hq routine list` — что заведено, тип, расписание/триггер, включена ли,
  когда последний раз отрабатывала, с каким результатом;
- управление: enable/disable/run-now/logs без правки launchd-плистов руками;
- типы задач (расширяемо): (1) внутренние функции ядра (скраббер транскриптов,
  автокоммит вольта, регенерация hot.md); (2) агентские работы платформы
  (автодрейн памяти, синтез, lint); (3) ПОЛЬЗОВАТЕЛЬСКИЕ агентские routine —
  например, разбор календаря, дайджест почты, регулярные отчёты; их пользователь
  описывает сам, платформа даёт запуск, гварды и наблюдаемость;
- стоимость на виду: для агентских routine — счётчик запусков и явная пометка
  «тратит токены»; гварды (порог/интервал) обязательны по умолчанию;
- единый журнал вместо россыпи логов (сейчас `~/.cache/wikipedik-autosync.log`
  живёт сам по себе);
- `hq doctor` знает о routine: сообщает, если джоба не отрабатывала дольше
  ожидаемого (иначе отказ снова будет молчаливым — см. урок про пульс).

Открытый вопрос — планировщик. launchd не может ходить в `~/Desktop` (TCC-гоча
2026-06-12), а вольт живёт там. Варианты: (а) перенести вольт из Desktop —
разблокирует launchd для всего; (б) триггеры от терминала и хуков агента (то,
что работает сейчас); (в) гибрид. Решать вместе с routine.

Файлы-якоря: core/setup/launchd.go, core/wiki/drain.go, core/wiki/autosynclog.go,
skills-stash/wiki/hooks/, shell/20-vaults.zsh.
