# Branch `public-portfolio-en` — English public repository

Preparation for **https://github.com/kantik001/grounded-horticulture** (dev.to, English audience).

Russian twin: [grounded-horticulture_ru](https://github.com/kantik001/grounded-horticulture_ru) · branch `public-portfolio`.

## Goals

| RU repo | EN repo |
|---------|---------|
| `grounded-horticulture_ru` | `grounded-horticulture` |
| Habr series (`docs/habr/`) | dev.to series (`docs/devto/` — phase 6) |
| Russian UI + RU agro KB | English UI + **see phase 4** for KB language |

## What stays the same as RU branch

- Platform code (Go, Python RAG, Docker, CI)
- Apache 2.0 license
- Demo corpus layout (`data/apple/sample_*.txt`, `data/demo_hr/`)
- Eval JSONL mechanics (questions may be translated in phase 4)
- Git hygiene: orphan `main` on publish (no journal corpus in history)

## What changes on this branch

See [TRANSLATION_PLAN_EN.md](./TRANSLATION_PLAN_EN.md) for phased checklist.

**Do not merge `public-portfolio-en` into `master`** (full private corpus).  
**Do not merge EN into RU** wholesale — cherry-pick platform fixes only.

## Publish EN repo (after phases 1–4 minimum)

```bash
git checkout public-portfolio-en
git checkout --orphan public-release-en
git add -A
git commit -m "Initial public release: grounded-horticulture (EN)."
git remote add public-en https://github.com/kantik001/grounded-horticulture.git
git push public-en public-release-en:main --force
git checkout public-portfolio-en
```

Before push:

- `git log -- .env` must be empty
- README demo GIFs present
- `docker compose up` smoke OK
- Cross-link to RU repo in README

## Remotes (after first publish)

| Remote | Repository |
|--------|------------|
| `origin` | private `grounded-horticulture` |
| `public-ru` | `grounded-horticulture_ru` |
| `public-en` | `grounded-horticulture` |

## Syncing fixes from private `main`

```bash
git checkout public-portfolio-en
git merge main -m "Merge platform fixes from main"
# resolve conflicts: keep EN translations in docs/webapp/config
```

## Cross-links in README

Each public README should contain:

```markdown
**Russian version:** [grounded-horticulture_ru](https://github.com/kantik001/grounded-horticulture_ru)
**English version:** [grounded-horticulture](https://github.com/kantik001/grounded-horticulture)
```
