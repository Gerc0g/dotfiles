# Company-level questions

Detailed walkthrough for filling TODOs in `~/Desktop/Prokectfiles/<co>/AGENTS.md`.

Walk these in order. ONE question per message. Apply Edit to AGENTS.md after each answer.

---

## Q1. Network constraints

**Section:** `## Coordinates` → `Network:` line

Ask:
> "Сетевые ограничения для ${CO}?
> 1. VPN required (через корпоративный VPN)
> 2. No VPN — публичные ресурсы
> 3. Custom (DNS / proxy / firewall — опиши)"

Apply: replace `Network: <!-- TODO ... --> TODO` with answer text.

---

## Q2. Stance

**Section:** `## Stance`

Ask:
> "Какая школа? Влияет на Definition of Done.
> 1. Willison-school — production, paid users, review mandatory (override DoD: MR с WHY, manual QA, integration test)
> 2. Steinberger-style — pet projects, atomic commits как safety net, минимум ceremony, YOLO mode на feature branches
> 3. Hybrid — описать"

Apply: replace `TODO: choose ...` line with выбранным вариантом + краткое обоснование (1 строка).

---

## Q3. Stack defaults

**Section:** `## Stack defaults` (таблица)

Apply 5 sub-questions подряд:

### Q3.1 Python
> "Python — какой менеджер пакетов?
> 1. `uv` (PLATFORM default)
> 2. `poetry`
> 3. `pip + venv`
> 4. `pdm`
> 5. Other"

### Q3.2 Node
> "Node — какой менеджер пакетов?
> 1. `pnpm` (PLATFORM default)
> 2. `npm`
> 3. `yarn`
> 4. `bun`"

### Q3.3 CI/CD
> "CI/CD платформа?
> 1. GitHub Actions
> 2. GitLab CI (`.gitlab-ci.yml`)
> 3. Jenkins
> 4. CircleCI
> 5. None / manual"

### Q3.4 Container registry
> "Container registry?
> 1. GitHub Container Registry (ghcr.io)
> 2. GitLab Container Registry
> 3. Docker Hub
> 4. Harbor (self-hosted)
> 5. ECR / GCR / ACR
> 6. None"

### Q3.5 Deployment
> "Deploy target?
> 1. Vercel
> 2. Kubernetes (managed / self-hosted)
> 3. Serverless (Lambda / Cloud Run)
> 4. VMs / bare metal
> 5. Mobile (App Store / Play Store)
> 6. None — local-only"

Apply: заполнить таблицу `## Stack defaults` row-by-row. Если совпадает с PLATFORM default — оставить `(matches default)` в ячейке.

---

## Q4. Product → namespace map

Before Q4, confirm the testing contract:

> "Подтверждаем общий test contract для ${CO}: repo-level `Makefile` — источник истины для тестов, targets `test`, `test-one`, `lint`, `format`, `typecheck`, `verify`, а `verify` не делает migrations/deploy.
> Есть company-specific override или оставляем default?"

Apply: если есть override — добавить bullet в `## Testing contract`; если default — оставить секцию как есть.

**Section:** `## Product → namespace map`

Ask first:
> "Все продукты ${CO} лежат в одном VCS namespace, или разделены по подгруппам?
> 1. Все в default `${NS}` — можно удалить эту таблицу
> 2. Разделены — заполним карту"

Если 1 — удалить всю таблицу + комментарий выше.

Если 2 — итеративно:
> "Назови первый продукт + его namespace (формат: `<product-name> | <ns>/<subpath>`).
> Например: `agents | dev/nrdsk_ai/agents`
> Type `done` когда все добавишь."

Apply: построчно добавляешь в таблицу `| <product> | \`<ns>\` |`.

---

## Q5. MR / PR conventions

**Section:** `## MR / PR conventions`

### Q5.1 Issue ID prefix
> "Префикс issue ID в commit title?
> 1. Да, всегда (e.g. `feat(NRDSK-123): ...`)
> 2. Нет — только conventional commits без ID
> 3. Опционально — если есть ticket"

### Q5.2 MR description sections
> "Какие секции обязательны в описании MR?
> 1. Summary / Why / How tested / Risks
> 2. Summary / Changes
> 3. Только commit-сообщение, без формального description
> 4. Custom — опиши секции"

### Q5.3 Approvers
> "Required approvers / groups?
> Просто перечисли (handles / team names) или `TODO когда узнаем` если ещё не известны."

Apply: заменить три TODO bullets ответами.

---

## Q6. Secrets & sensitive data

**Section:** `## Secrets & sensitive data`

### Q6.1 PII handling
> "PII / sensitive data в репах ${CO}?
> 1. Нет — обычные dev данные
> 2. Да, есть PII — нужна осторожность (no echo, no logs)
> 3. Compliance — GDPR / HIPAA / SOC2 → конкретный compliance"

### Q6.2 Extra files to never commit
> "Что нельзя коммитить помимо .env? (e.g. fixtures с реальными ID, dump.sql, credentials.json)
> Перечисли или `none`."

Apply: заменить 2 TODO bullets.

---

## Q7. Communication

**Section:** `## Communication`

### Q7.1 Key people
> "Ключевые люди в ${CO}? handles / nicknames (Slack/GitLab/GitHub).
> Если ещё не выясняли — `TODO когда узнаем`."

### Q7.2 Review tagging
> "Кого тегать на code review по типу изменения? (e.g. `backend → @user-a, frontend → @user-b, infra → @user-c`).
> Если не известно — `TODO`."

### Q7.3 Escalation
> "Escalation path для security / infra / release incidents?
> Если не известно — `TODO`."

Apply: заменить 3 TODO bullets.

---

## Q8. External references

**Section:** `## External references`

Подряд 5 коротких:

### Q8.1 Documentation
> "Где документация? URL Confluence / Notion / GitLab Wiki?
> Если нет — `none yet`."

### Q8.2 Issue tracker
> "Issue tracker URL? (GitLab Issues / Jira / Linear / GitHub Issues)"

### Q8.3 Production URLs
> "Prod URLs / dashboards / monitoring? Можно перечислить через ;"

### Q8.4 Staging URLs
> "Staging URLs? Если совпадают с deploy target из Q3.5 — `same as deploy target`."

### Q8.5 Incident runbook
> "Где incident runbook? URL или `none yet`."

Apply: заменить 5 TODO bullets.

---

## Final step

После всех answers:

1. Run: `grep -n TODO ~/Desktop/Prokectfiles/${CO}/AGENTS.md`
2. Если осталось > 0 — list какие, ask "хочешь доделать или оставить как есть?"
3. Run: `git status` в `~/dotfiles` (чтобы видеть нет ли изменений template, если случайно правился) + в `~/Desktop/Prokectfiles/${CO}/` (но это не git repo обычно)
4. Suggest commit message in conventional format (Russian description):
   > "Готово. Suggested commit:
   > `chore(${CO}): заполнить TODO в company AGENTS.md`
   > Применить commit? (только с твоего ask)"

5. **Не выполнять `git commit`** пока пользователь не скажет явно.
