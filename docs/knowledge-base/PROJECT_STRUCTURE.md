# Структура проекта `doctor_gardens_ai`

Ниже — карта проекта по текущему состоянию репозитория: где какой файл и за что отвечает.

> Подробные разборы отдельных файлов: см. [README.md](./README.md) в этой папке.

## Корень проекта

- `.env.example` — пример переменных окружения для локального запуска и прод-конфига.
- `.gitignore` — исключения для Git (артефакты, секреты, временные файлы).
- `.dockerignore` — исключения при сборке Docker-образов.
- `README.md` — общее описание системы, запуск, API и архитектурный обзор.
- `Makefile` — удобные команды для разработки (тесты/запуск/утилиты).
- `docker-compose.yml`, `Dockerfile.*` → [docker-overview.md](./docker-overview.md)

## `docs/knowledge-base/` (эта папка)

- `README.md` — оглавление базы знаний.
- `PROJECT_STRUCTURE.md` — этот файл (карта проекта).
- `classifier-api_server.md` — разбор Python-сервиса `classifier/api_server.py`.
- `classifier-apple_classifier.md` — разбор CV-модели `classifier/apple_classifier.py`.
- `classifier-registry.md` — фабрика моделей `classifier/registry.py`.
- `classifier-train_classifier.md` — обучение `classifier/train_classifier.py`.
- `github-ci.yml.md` — разбор `.github/workflows/ci.yml`.
- `migrations-overview.md` — все SQL-миграции `migrations/*.sql`.
- `rag-crops_config.md`, `rag-vector_store.md`, `rag-retrieval.md`, `rag-verifier.md` — модули `rag/`.
- `scripts-overview.md` — утилиты `scripts/`.
- `tests-overview.md` — pytest в `tests/`.
- `webapp-overview.md` — фронтенд `webapp/`.
- `server-overview.md`, … — backend Go.
- `config-overview.md`, `docker-overview.md`, `data-pipeline.md`, `quality-eval-and-rag-logs.md` — конфиг, Docker, данные, план качества.

## `.github/workflows`

- `ci.yml` — GitHub Actions CI: тесты и проверка сборки. → [github-ci.yml.md](./github-ci.yml.md)

## `classifier/` (Python: CV-классификация)

- `api_server.py` — HTTP API классификатора (эндпоинты для фото/health). → [classifier-api_server.md](./classifier-api_server.md)
- `apple_classifier.py` — модель и логика инференса. → [classifier-apple_classifier.md](./classifier-apple_classifier.md)
- `registry.py` — реестр моделей по `crop_id`. → [classifier-registry.md](./classifier-registry.md)
- `train_classifier.py` — обучение на датасете. → [classifier-train_classifier.md](./classifier-train_classifier.md)
- `requirements.txt` — Python-зависимости сервиса классификации.

## `config/` (доменные и prompt-конфиги)

→ [config-overview.md](./config-overview.md)

- `crops.json`, `prompts.json`, `few_shot.json`, `onboarding.json`, `article_titles.json`

## `data/` (база знаний для RAG)

### `data/apple/`

- `article1.txt` — статья/фрагменты знаний по яблоне.
- `article2.txt` — статья/фрагменты знаний по яблоне.
- `article3.txt` — статья/фрагменты знаний по яблоне.

### `data/pear/`

- `README.txt` — заметка/заглушка по заполнению контента для груши.

### `data/plum/`

- `README.txt` — заметка/заглушка по заполнению контента для сливы.

## `docs/` (обучающие и плановые документы)

- `ROADMAP.md` — общий план развития проекта по фазам.
- `knowledge-base/` — база знаний по коду (эта папка).
- `LEARNING_SESSION_1.md` — итоги и выводы сессии 1.
- `LEARNING_SESSION_2.md` — итоги и выводы сессии 2.
- `LEARNING_SESSION_3.md` — итоги и выводы сессии 3.
- `LEARNING_SESSION_5.md` — итоги и выводы сессии 5.
- `LEARNING_SESSION_6.md` — итоги и выводы сессии 6.

## `migrations/` (SQL-миграции БД)

→ Общий разбор: [migrations-overview.md](./migrations-overview.md)

- `001_init.sql` — базовая схема (пользователи, сессии, сообщения).
- `002_crop_id.sql` — расширение схемы под мультикультуру (`crop_id`).
- `003_feedback_analytics.sql` — таблицы feedback/аналитики.

## `rag/` (Python: retrieval и верификация)

- `__init__.py` — пакетный маркер (пустой/минимальный, в базе знаний не разбирается).
- `crops_config.py` → [rag-crops_config.md](./rag-crops_config.md)
- `vector_store.py` → [rag-vector_store.md](./rag-vector_store.md)
- `retrieval.py` → [rag-retrieval.md](./rag-retrieval.md)
- `verifier.py` → [rag-verifier.md](./rag-verifier.md)
- `__pycache__/` — автокэш Python (см. FAQ в чате / не коммитить).

## `scripts/` (утилиты)

→ [scripts-overview.md](./scripts-overview.md)

- `reindex_rag.py` — принудительная переиндексация RAG-базы.
- `smoke.sh` — smoke-проверки API для Linux/macOS.
- `smoke.ps1` — smoke-проверки API для Windows PowerShell.

## `server/` (backend API)

→ Начните с [server-overview.md](./server-overview.md)

| Файл | Статья |
|------|--------|
| `main.go` | [server-overview.md](./server-overview.md) |
| `auth_telegram.go`, `middleware.go`, `ratelimit.go` | [server-auth-and-limits.md](./server-auth-and-limits.md) |
| `messenger.go`, `chat_session.go`, `postgres_store.go` | [server-chat-and-db.md](./server-chat-and-db.md) |
| `rag_chat.go` | [server-rag_chat.md](./server-rag_chat.md) |
| `admin.go`, `onboarding.go`, `feedback.go`, `analytics_store.go`, `crops.go` | [server-admin-and-ux-api.md](./server-admin-and-ux-api.md) |
| `go.mod`, `go.sum` | зависимости Go |
| `*_test.go` | [tests-overview.md](./tests-overview.md) |

## `tests/` (Python-тесты)

→ [tests-overview.md](./tests-overview.md)

- `conftest.py` — pytest: `PYTHONPATH` на корень проекта.
- `test_crops_config.py` — тесты `rag/crops_config.py`.
- `test_verifier.py` — тесты `rag/verifier.py`.
- `requirements-test.txt` — pytest + langchain-core (без Chroma/PyTorch).

## `webapp/` (клиентский интерфейс)

→ [webapp-overview.md](./webapp-overview.md)

- `index.html` — Telegram Web App: чат, фото, онбординг, feedback.
- `admin.html` — админка: upload `.txt`, reindex (Basic auth).
- `nginx.conf` — прокси `/api/` → Go, раздача HTML.

---

## Как лучше изучать код (рекомендуемый порядок)

1. `README.md` → быстрый контекст по архитектуре.
2. `docker-compose.yml` → как связаны сервисы.
3. [server-overview.md](./server-overview.md) → маршруты и старт.
4. [rag-vector_store.md](./rag-vector_store.md) → [rag-retrieval.md](./rag-retrieval.md) → `server/rag_chat.go` → ядро RAG.
5. [classifier-api_server.md](./classifier-api_server.md) + [classifier-apple_classifier.md](./classifier-apple_classifier.md) → CV-ветка.
6. `migrations/*.sql` + `server/postgres_store.go` → БД и персистентность.
7. `tests/` и `server/*_test.go` → что считается корректным поведением.
