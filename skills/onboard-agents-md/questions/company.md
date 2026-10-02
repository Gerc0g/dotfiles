# Company-level questions

Use the canonical document and item IDs from `hq entity show <company> --json`.
These headings match `templates/AGENTS.md.company.tmpl`. First reuse verified
HQ coordinates, existing context and prior session answers. Ask only unresolved
questions, one at a time, within any requested item focus. Save and confirm
through the protocol in SKILL.md; generated baseline rules are not human choices.

## Network

**Section:** `## Coordinates`, the `Network:` line.

Ask: «Какие сетевые ограничения у компании: публичный доступ, VPN или особые
DNS/proxy/firewall? Что нужно агенту для работы?»

Replace that line's exact placeholder. Do not change VCS/SSH/config coordinates
based on this answer. The backend `network` item covers this line.

## Workflow and stance

**Sections:** `## Stance`, `## Git workflow`, `## Stack overrides`.

Ask in separate turns, only where answers are missing:

1. «Какой режим работы: обязательный review для production, минимальные правила
   для личных проектов или свой вариант? Какие требования к завершению задачи?»
2. «Есть отличия от описанного Git workflow: ветки, review, формат коммитов?»
3. «Какие отличия от базового стека: версии, package managers, CI/CD, registry,
   среда развёртывания? Если отличий нет, так и зафиксируем».

`Stack overrides` is prose/bullets, not a `Stack defaults` table. Preserve
baseline defaults; record only known differences or an explicitly confirmed
absence of overrides. No answer is permission to bypass platform safety rules.

The backend `workflow` item spans all three sections. A stance answer alone
does not confirm the existing Git rules or stack text.

## Testing contract

**Section:** `## Testing contract`.

Read the existing contract: Make targets are the executable interface and
`verify` is local and non-destructive. Ask for company-specific exceptions only
if the user needs them. Do not create or modify Makefiles in this walkthrough.
This section is not a separate manual item unless the backend returns one.

## Product namespaces

**Section:** `## Product → namespace map`.

Ask: «Все продукты используют namespace из конфигурации компании или есть
исключения? Для исключений укажи продукт и namespace».

Fill the existing table for known exceptions. If the user confirms one shared
namespace, replace the placeholder row/comment with that fact. This is context
documentation; it does not change `.company-config` or `.product-config`.

## Data and secrets

**Section:** `## Secrets`.

Ask: «Какие чувствительные данные есть в проектах, какие правила обращения с
ними и какой backend секретов выбран, если он уже нужен?»

Follow up only for unresolved details. Record backend names and restrictions,
never credentials. No backend is created by company onboarding. Do not assume
1Password, invent a vault name, or reconfigure existing credentials.

The backend `data` item covers the complete Secrets section, including existing
rules and the PII placeholder. Unknown requirements remain TODO.

## References

**Section:** `## External references`.

Ask: «Где документация компании? Какие рабочие ссылки стоит добавить: tracker,
runbooks, monitoring? Если их пока нет, можно явно это указать».

Replace the Docs placeholder and add only supplied/reliably verified links in
this existing section. Do not fabricate Communication or MR/PR conventions
sections that are absent from the current template.

## Finish

Read the card again and report its status and outstanding items. Any read-only
automatic `coordinates` item is owned by HQ. Stopping questions, resolving
placeholders, and confirming human decisions are different actions. No
automatic commit or setup command follows completion.
