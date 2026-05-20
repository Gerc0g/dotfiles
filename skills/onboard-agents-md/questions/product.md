# Product-level questions

Walkthrough TODOs in `~/Desktop/Prokectfiles/<co>/<prod>/AGENTS.md`.

ONE question per message. Apply Edit immediately.

---

## Q1. What this product does

**Section:** `## What this is (one line)`

Ask:
> "Что делает product `${PROD}` одной строкой? (что строит / для кого / какая ценность)"

Apply: replace `<TODO: one sentence...>` with answer.

## Q2. Status

Ask:
> "Текущий статус продукта?  
> 1. production — живой продукт с users  
> 2. staging — почти production, последние проверки  
> 3. experimental — POC, может быть выкинут  
> 4. legacy — поддерживается но не развивается"

Apply: replace `Status: <TODO: ...>` line.

---

## Q3. Repos and roles

**Section:** `## Repos and roles` (таблица)

Read the directory listing first:
```bash
ls -d */ | grep -v AGENTS.md
```

Затем для каждого репо:
> "Репо `${REPO}` — что делает (одна строка) и tech stack hint?  
> Пример: `API gateway, request validation, rate limit | FastAPI`"

Apply: построчно заполнить таблицу `| ${REPO} | <role> | <stack hint> |`.

---

## Q4. Data flow

**Section:** `## Data flow (only if non-obvious)`

Ask:
> "Data flow между репами этого product — есть нетривиальный? Например один отправляет события, другой потребляет, третий хранит state?  
> 1. Yes — описать (попроси нарисовать 2-5 строк ASCII или текст)  
> 2. No — репы независимые / простой monorepo"

Если 2 — удалить всю секцию + комментарий.

Если 1 — попросить:
> "Опиши flow словами или ASCII. Например:  
> `client → barrier (gateway) → cortex (router) ─┬→ nerve (events)`  
> `                                              ├→ vox (voice)`  
> `                                              └→ cerebellum (state)`"

Apply: replace placeholder text with answer.

---

## Q5. Local development

**Section:** `## Local development`

Ask 4 подряд:

### Q5.1 Setup steps (numbered)
> "Какие шаги от cold до running product? (e.g. `dev-stack up` → `db-create` → `cd repoA && uv run uvicorn` → `cd repoB && pnpm dev`)"

### Q5.2 Critical repos
> "Какие репы критичные для dev session (без них ничего не работает)?"

### Q5.3 Mockable repos
> "Какие репы можно НЕ запускать или замокать?"

### Q5.4 Health check
> "Как проверить что product healthy? (curl endpoint / port check / specific log line)"

Apply: заменить 4 TODO bullets + setup steps numbered list.

---

## Q6. Cross-repo conventions

**Section:** `## Cross-repo conventions`

### Q6.1 Shared schemas location
> "Где живут shared schemas / DTOs / proto-файлы?  
> 1. Отдельный repo (назови какой)  
> 2. Inline в каждом репо — duplicated  
> 3. None — все API изолированы"

### Q6.2 Consumer repos
> "Когда меняешь API в одном репе — какие repos нужно обновлять синхронно? Перечисли relationships."

Apply: заменить TODO bullets.

---

## Q7. Procedural workflows

**Section:** `## Procedural workflows`

Ask:
> "Есть ли повторяющиеся multi-step операции в этом product, которые стоит документировать?  
> 1. Adding new service / module / component  
> 2. Rolling out cross-repo contract change  
> 3. Specific deployment workflow  
> 4. Другие — опиши  
> 5. Нет, не нужно — удалить секцию"

Если есть — попросить пошагово (1-N steps) для каждого workflow. Apply: заполнить subsections.

---

## Q8. Boundaries

**Section:** `## Boundaries (product-specific)`

### Q8.1 Don't touch
> "Что в этом product нельзя трогать без explicit approval? (e.g. shared schemas dir, migrations, infra configs)"

### Q8.2 Ask first
> "Операции которые spanning multiple repos и требуют ask? (e.g. version bump shared deps, breaking change в shared DTO)"

Apply: заменить TODO bullets.

---

## Q9. References

**Section:** `## References`

3 подряд:
> "Product Confluence/Notion page URL? (или `none`)"
> "Architecture diagram path? (или `none`)"
> "Production incident runbook URL? (или `none`)"

Apply: заменить 3 TODO bullets.

---

## Final step

1. `grep -n TODO AGENTS.md` — count remaining
2. `git diff AGENTS.md` (если product dir в git repo) — preview
3. Suggested commit (если в git):
   > `chore(${CO}/${PROD}): заполнить product AGENTS.md`
4. NEVER auto-commit.
