# Secrets And Environment

Reference for 1Password, `op` CLI, direnv, `.envrc`, `.env` files, env vars,
credentials, tokens, API keys, and secret handling.

## Rules

- No plaintext secrets in git.
- Do not edit `.env*` or `.envrc` without explicit user ask.
- Prefer `secret add <VAR> <value>` for new repo/company secrets.
- Assume runtime code reads from environment (`os.environ`, `process.env`,
  framework settings), not from committed secret files.
- `.env.example` is documentation only.

## 1Password

Vault convention:

- company vault: `Work-<company-slug>`;
- item name: env var name;
- field: `credential`.

Useful commands:

```bash
secret signin
secret add <VAR> <value>
secret get op://Work-<company>/<VAR>/credential
secret list <vault>
```

## direnv

`direnv` loads `.envrc` on `cd`. Company/product/repo files may chain.

After a legitimate `.envrc` change, tell the user to run:

```bash
direnv allow
```

## When Blocked

- If `op` is not signed in, ask the user to run `secret signin`.
- If 1Password asks for biometric approval too often, do not bypass security in
  repo code. Adjust 1Password/CLI/session policy with the user.
- If a repo needs a new env var, document the variable and tell the user the
  exact `secret add` command.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` sections "Secrets" and
"Environment loading".
