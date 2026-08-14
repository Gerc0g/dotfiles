# Secrets And Environment

Reference for 1Password, `op` CLI, direnv, `.envrc`, `.env` files, env vars,
credentials, tokens, API keys, and secret handling.

> **STATUS 2026-08-14:** слой секретов переделывается. `hq secret` — заглушка
> (работает только `secret signin`), `secret-cache` удалён; активные
> secret-cache строки в существующих `.envrc` закомментированы с маркером
> `[secret stub]` и бэкапами `.envrc.bak-secret-purge-*`. Команды `secret add`
> и схема wiring ниже описывают ПРЕЖНИЙ слой — контракт, который сохранит
> новая реализация (см. докстринг `core/secret`). Правила из раздела Rules
> действуют без изменений.

## Rules

- No plaintext secrets in git.
- Do not edit `.env*` or `.envrc` without explicit user ask.
- Prefer `secret add --repo <VAR> <value>` for repo secrets,
  `secret add --product <VAR> <value>` for product secrets, and
  `secret add --company <VAR> <value>` for company-wide secrets.
- Assume runtime code reads from environment (`os.environ`, `process.env`,
  framework settings), not from committed secret files.
- `.env.example` is documentation only.

## 1Password

Vault convention:

- company vault: `Work-<company-slug>`;
- item name includes the narrowest needed scope:
  - company: `_company__<VARNAME>`;
  - product: `<product>__<VARNAME>`;
  - repo: `<product>__<repo>__<VARNAME>`;
- field: `credential`.

Useful commands:

```bash
secret signin
secret add --repo <VAR> <value>
secret add --product <VAR> <value>
secret add --company <VAR> <value>
secret name --repo <VAR>
secret envline --repo <VAR>
secret list <vault>
```

## direnv

`direnv` loads `.envrc` on `cd`. Company/product/repo files chain with
`source_up`.

Scope rules:

- company `.envrc`: git identity and company-wide env only;
- product `.envrc`: product-wide env shared by all repos in a product;
- repo `.envrc`: repo runtime env and repo-only secrets.

`secret add` appends a `secret-cache` backed export to the `.envrc` for the
selected scope. Cached values live under `~/.cache/dotfiles/secrets` with file
mode `600` and default TTL of 30 days.

After a legitimate `.envrc` change, tell the user to run:

```bash
direnv allow
```

## When Blocked

- If `op` is not signed in, ask the user to run `secret signin`.
- If 1Password asks for biometric approval too often, do not bypass security in
  repo code. Adjust 1Password/CLI/session policy with the user.
- If a repo needs a new env var, document the variable and tell the user the
  exact scoped `secret add --repo ...` command.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` sections "Secrets" and
"Environment loading".
