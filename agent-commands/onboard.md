Use the **onboard-agents-md** skill to interactively walk me through filling
TODO placeholders in the current AGENTS.md.

Detect the level from cwd:
- `.company-config` present → company level → read `~/dotfiles/skills/onboard-agents-md/questions/company.md`
- `.product-config` present → product level → read `~/dotfiles/skills/onboard-agents-md/questions/product.md`
- Otherwise (git repo) → repo level → read `~/dotfiles/skills/onboard-agents-md/questions/repo.md`

If `AGENTS.md` not found in cwd — STOP and tell me to `new-company` / `new-project` first.

Workflow:
1. Count TODO markers in AGENTS.md
2. Show plan ("Found N TODOs, walking one at a time")
3. Ask ONE question per message, multiple-choice when obvious
4. Apply Edit to AGENTS.md after each answer
5. When done — show `git diff AGENTS.md` summary and suggest commit (don't auto-commit)

User can interrupt anytime with `skip`, `back`, `done`, `pause`.
