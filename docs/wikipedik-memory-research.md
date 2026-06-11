# Память LLM/кодинг-агентов: исследование подходов и рынка (2026-06-11)

Deep-research прогон: 5 поисковых направлений, 23 источника, 114 извлечённых
утверждений, 25 верифицированы адверсариально (3 голоса), 23 подтверждены,
2 отклонены. Дополняет [wikipedik-roadmap.md](./wikipedik-roadmap.md).

## Главный вывод

**Рынок 2025–2026 сошёлся на архитектуре WikiPedik.** Три независимых вендора
пришли к "plain-файлы + маленький авто-инжектируемый горячий индекс +
on-demand глубокое чтение":

- **Claude Code auto memory** (default с v2.1.59): `MEMORY.md` авто-грузится
  (200 строк / 25KB), topic-файлы читаются по требованию; память кейится
  per-git-repo, локацию можно перенаправить через `autoMemoryDirectory`.
- **VS Code / Copilot agents**: три файловых scope (user / repo / session),
  авто-инжект первых 200 строк user memory.
- **Anthropic memory tool (API)**: полностью client-side и файловый; Anthropic
  официально позиционирует файловую память как примитив "just-in-time context
  retrieval".

Контр-кейс: **Cursor намеренно удалил авто-Memories** (sidecar-модель,
наблюдавшая чат) в 2.1.x в пользу явных файловых Rules — рыночное
доказательство, что авто-захват без human-in-the-loop ломает доверие.
Архитектурные ставки WikiPedik (filesystem-first, explicit capture, куратор)
подтверждены; менять фундамент не нужно — нужно докрутить механики.

## Что взять (по убыванию ROI)

### 1. Глубина чтения: hot.md → индекс + on-demand протокол (слабое место a)

- hot.md консервативен: рынок даёт бюджет ~200 строк / 25KB против наших ~50.
  Вырастить до ~150–200 строк и сменить формат: **однострочные указатели на
  topic-страницы**, а не самодостаточные выжимки (паттерн MEMORY.md).
- В SessionStart-чекпойнт добавить протокол Anthropic memory tool:
  *"ALWAYS VIEW YOUR MEMORY DIRECTORY BEFORE DOING ANYTHING ELSE...
  ASSUME INTERRUPTION"* — агент сам дочитывает нужные страницы по требованию.
- `.claude/rules/` нативно поддерживает **симлинки** (с детекцией циклов) и
  **path-scoped загрузку** (frontmatter `paths:` — правило подгружается только
  при работе с матчащими файлами). Это даёт "selective deep read" curated-
  страниц по подсистемам кода без embeddings. Известные баги: #21858 (paths в
  user-level), #23478 (Read но не Write), #23569 (worktrees).

### 2. Anti-staleness: цитаты + JIT-верификация (слабое место c)

Паттерн GitHub Copilot memory (public preview 2026-01) — единственный
верифицированный готовый механизм борьбы со стейлом:

- Схема памяти: `{subject, fact, citations file:line, reasoning}`.
- **Just-in-time верификация вместо offline-курации**: перед применением
  урока агент проверяет процитированные места в коде; если код противоречит —
  сохраняет исправленную версию. GitHub явно рассматривал offline curation
  service и отказался.
- Авто-экспирация неиспользуемых памятей через 28 дней.

Для WikiPedik: обязательное поле `citations:` (file:line / commit) в схеме
`lesson-append`, verify-before-apply шаг у worker (исправление → новая
запись в inbox), куратору — формальный сигнал "память протухла".

### 3. Связность: Zettelkasten-линки + эволюция памяти

- **A-MEM (NeurIPS 2025)**: атомарные заметки + LLM-генерируемые
  keywords/tags/links (Zettelkasten) + "memory evolution" — новый урок
  триггерит обновление атрибутов/контекста связанных старых записей.
  Переносимо как frontmatter + wikilinks; референс использует
  ChromaDB+embeddings, но в filesystem-first варианте линковку делает
  LLM-куратор при drain.
- **HippoRAG (NeurIPS 2024)**: граф связей между заметками даёт multi-hop
  retrieval, недоступный плоскому поиску (в оригинале KG + Personalized
  PageRank). Лёгкая версия = wikilinks + обход графа куратором.
  (Цифра "+20% над SOTA" — бенчмарк против SOTA-2024, уже превзойдена.)

### 4. Метрики и forgetting (слабое место d)

- Трекинг "last used" для curated-записей; кандидаты на архивацию по образцу
  28-дневной экспирации Copilot (у нас — мягче: предлагать в lint, не удалять).

### 5. Self-directed память (идея MemGPT)

- MemGPT: LLM сам управляет иерархией памяти через function calls, а не
  получает всё пассивной инъекцией. У нас уже есть зачатки (lesson-append,
  wiki-context-pack) — усилить: память как инструменты, инъекция только
  индекса.

### 6. Интеграция Claude Code auto memory

- `autoMemoryDirectory` в settings.json → можно направить в staging-зону
  вольта, которую дренирует куратор (вместо двух параллельных памятей).
  Открытый вопрос: не конфликтует ли per-git-repo keying c privacy firewall
  по компаниям — развести staging-зоны по компаниям до включения.

## Что игнорировать (и почему)

| Что | Почему |
|---|---|
| Vector DB / embeddings retrieval | A-MEM/HippoRAG показывают пользу **связей/графа**, не векторов как таковых; нарушает no-heavy-infra. Порог, где локальный embedding-индекс окупится, — открытый вопрос (сотни заметок пока не предел). |
| Hosted-платформы (Letta, Mem0, Zep) как замена | Vendor lock-in; вся переносимая ценность реализуема markdown+куратором. NB: как *дополнение* (MCP-слой) не исследованы — пробел данных, не приговор. |
| Полностью автоматический sidecar-майнинг сессий | Cursor прожил полный цикл и откатился; auto-захват без approve ломает доверие. Наш `agent-history-ingest` должен оставаться curator-in-the-loop. |

## Таксономия для словаря проектирования

Survey "Memory in the Age of AI Agents" (arXiv 2512.13564, дек 2025):
функции памяти = factual / experiential / working; динамика = formation →
evolution → retrieval. Цикл WikiPedik ложится на эту рамку точно:
inbox=formation, curation/synthesize=evolution, hot.md/context-pack=retrieval.

## Оговорки

- Mem0, Zep/Graphiti, Cognee, LangMem, OpenMemory/MCP-серверы, beads, ChatGPT
  memory, Windsurf — **не покрыты** выжившими верифицированными claims;
  отсутствие в рекомендациях = пробел данных. Отдельный раунд при
  необходимости.
- Copilot memory и VS Code memory — public preview, могут измениться;
  JIT-верификация Copilot prompt-based (best-effort, не гарантия).
- Отклонены проверкой (не опираться): "ровно три формы памяти в survey",
  "HippoRAG 10–30x дешевле IRCoT, заменяет агентные retrieval-циклы".

## Открытые вопросы

1. MCP-слой semantic-поиска поверх markdown (read-only) — цена эксплуатации
   vs чистый grep+wikilinks?
2. LLM-линковка без embeddings: рабочий масштаб и порог окупаемости
   локального индекса (например sqlite-vec)?
3. `autoMemoryDirectory` vs privacy firewall по компаниям.
4. Количественные метрики полезности памяти (доля уроков, процитированных в
   последующих сессиях; precision JIT-верификации) — методики в рынке нет.

## Ключевые источники

- arXiv 2512.13564 — survey "Memory in the Age of AI Agents" (дек 2025)
- arXiv 2502.12110 — A-MEM (NeurIPS 2025)
- arXiv 2310.08560 — MemGPT
- arXiv 2405.14831 — HippoRAG (NeurIPS 2024)
- code.claude.com/docs/en/memory — Claude Code auto memory + rules
- platform.claude.com/.../memory-tool — Anthropic memory tool
- github.blog — "Building an agentic memory system for GitHub Copilot"
- code.visualstudio.com/docs/copilot/agents/memory — VS Code agents memory
- cursor.com/changelog + forum.cursor.com — запуск и удаление Cursor Memories
