# Сессия 6 — тесты и CI

## Что сделали

1. **Go unit-тесты** (`server/*_test.go`):
   - Telegram `initData` (было)
   - RAG: `verifyRAGAnswer`, числа, источник
   - `normalizeCropID`, `safeFilename`
2. **Python pytest** (`tests/`):
   - `rag/verifier.py`, `rag/crops_config.py`
   - без Torch/Chroma (быстрый CI)
3. **Smoke** — `scripts/smoke.ps1`, `scripts/smoke.sh` (health, crops, session, onboarding)
4. **GitHub Actions** — `.github/workflows/ci.yml` (go test, pytest, docker build server/webapp)

## Зачем (обучение)

| Идея | Реализация |
|------|------------|
| **Пирамида тестов** | Unit без Docker; smoke — после `compose up` |
| **CI на каждый PR** | GitHub Actions ловит поломки до merge |
| **Изоляция ML** | CI не гоняет полный classifier (тяжёлый образ) |
| **Контракт API** | Smoke проверяет `success` и публичные маршруты |

## Локально

```powershell
# Go
cd server
go test -v ./...

# Python
pip install -r tests/requirements-test.txt
pytest tests/ -v

# Smoke (Docker должен быть запущен, TELEGRAM_AUTH_DISABLED=true)
.\scripts\smoke.ps1
```

## CI

При push/PR на `master` / `feature/*` запускаются job'ы `go-test`, `python-test`, `docker-build`.

## Следующая сессия (4)

Контент: больше статей на яблоню, обучение CV, метрики.
