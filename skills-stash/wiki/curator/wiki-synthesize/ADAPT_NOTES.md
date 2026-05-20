# Adapt notes — wiki-synthesize

Changes from SKILL.md_RAW.md:
- Drop dependency on `_meta/taxonomy.md` (we don't have it)
- Target patterns by tag co-occurrence across repos in same product/company instead
- Output: `~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/shared/patterns/<pattern-slug>.md`
- Input: `_synthesis-candidates.md` (queued by inbox-drain) + all `repos/*/lessons.md`, `gotchas.md`
- Add backlinks from constituent repo lessons to the new pattern page
- Update `<co>/<prod>/index.md` "Patterns" section after creating pattern
