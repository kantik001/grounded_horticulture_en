# 🍏 doctor_gardens_ai — помощник садовода

Telegram Web App + AI: **фото** → классификация болезней; **текст** → ответы по статьям (RAG). Оркестрация и LLM — **Go**, CV и векторный поиск — **Python**.

Полный план развития: [`docs/ROADMAP.md`](docs/ROADMAP.md).  
База знаний по коду (для команды): [`docs/knowledge-base/README.md`](docs/knowledge-base/README.md).

## Описание

- Распознавание яблока/листа (**MobileNetV2** в Python) и рекомендации по фото (**Go** → LLM или шаблоны).
- Текстовые вопросы по статьям из **`data/`**: Python **только retrieval** (`/rag/context`), ответ собирает **Go** + LLM + верификация.
- **Telegram auth**: Web App передаёт `initData`, Go проверяет подпись бота (см. `server/auth_telegram.go`).

## Архитектура

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────────────┐
│  Telegram Web   │────▶│   Go Server      │────▶│ Python (Flask)              │
│  App + initData │◀────│  auth, rate limit│     │  /classify — CV             │
└─────────────────┘     │  /message, /chat │────▶│  /rag/context — Chroma      │
                          └────────┬────────┘     └─────────────────────────────┘
                                   │
                                   ▼  LLM (OpenRouter / OpenAI-compatible)
                            ┌──────────────┐
                            │  LLM API     │
                            └──────────────┘
```

## Структура каталогов

```
union_ai_apple_project/
├── classifier/       # Flask: /classify, /chat
├── rag/              # векторное хранилище, LLM-ответ, верификация
├── data/             # .txt статьи для RAG
├── server/           # Go: /classify, /chat, LLM для подсказок по фото
├── webapp/           # index.html, nginx
└── docker-compose.yml
```

## Технологии

### Python-сервис
- **MobileNetV2** + **PyTorch** — классификация изображений
- **Flask** — HTTP API (`/classify`, `/rag/context`, `/health`)
- **LangChain + Chroma + HuggingFace embeddings** — поиск фрагментов статей из `data/`

### Серверная часть (Go)
- **Gin** - веб-фреймворк
- **HTTP Client** - взаимодействие с Python сервисом и LLM API

### Клиентская часть
- **Telegram Web App** - кроссплатформенное приложение
- **HTML/CSS/JavaScript** - нативный веб-интерфейс

## Классы для распознавания

Модель обучена распознавать следующие категории:
1. `healthy_apple` - Здоровое яблоко
2. `apple_scab` - Парша яблони
3. `black_rot` - Чёрная гниль
4. `cedar_apple_rust` - Кедрово-яблоневая ржавчина
5. `healthy_leaf` - Здоровый лист
6. `powdery_mildew` - Мучнистая роса
7. `fire_blight` - Бактериальный ожог
8. `bitter_rot` - Горькая гниль
9. `blue_mold` - Голубая плесень
10. `brown_rot` - Бурая гниль

## Процесс работы

**Фото:** пользователь в Web App делает снимок → `POST /classify` (Go → Python CV) → Go дополняет ответом LLM или шаблоном.

**Текст:** вопрос в чате → `POST /api/message` или `/api/chat` → Go → Python `/rag/context` → Go + LLM → ответ со ссылкой на источник.

**Безопасность:** все API, кроме `GET /health`, требуют заголовок `X-Telegram-Init-Data` (кроме режима `TELEGRAM_AUTH_DISABLED=true` для локальной разработки).

## Установка и запуск

### Вариант 1: Локальная установка (для разработки)

#### 1. Установка зависимостей Python

```bash
cd classifier && pip install -r requirements.txt
```

**Примечание:** Для установки PyTorch с поддержкой CUDA посетите https://pytorch.org/get-started/locally/

#### 2. Настройка переменных окружения

Скопируйте файл `.env.example` в корень проекта:

```bash
cp .env.example .env
```

Отредактируйте `.env` при необходимости:

См. **`.env.example`**: `TELEGRAM_BOT_TOKEN`, `LLM_API_KEY`, `DATABASE_URL`, `UPLOAD_DIR`, `CLASSIFIER_URL`, `CLASSIFIER_RAG_URL`, `CORS_ALLOWED_ORIGINS`, `RATE_LIMIT_REQUESTS_PER_MINUTE`.

**Локальная разработка без Telegram:** в `.env` задайте `TELEGRAM_AUTH_DISABLED=true` (только dev, не для продакшена).

**Продакшен:** укажите `TELEGRAM_BOT_TOKEN` от @BotFather; в `CORS_ALLOWED_ORIGINS` добавьте URL вашего Web App (например `https://your-domain.com`).

#### 3. Запуск Python сервиса классификации

```bash
cd classifier
python api_server.py
```

Сервис запустится на порту 5000.

**Важно:** Если файл модели не найден, сервис запустится с базовыми весами ImageNet. 
Для загрузки собственных весов:
1. Обучите модель через `train_classifier.py`
2. Поместите файл модели в папку `models/`
3. Укажите путь в переменной `MODEL_PATH`

#### 4. Установка зависимостей Go

```bash
cd server
go mod download
```

#### 5. Запуск Go сервера

```bash
cd server
go run main.go
```

Сервер запустится на порту 8080 и автоматически загрузит `.env` файл из директории `server/`.

#### 6. Размещение Web App

Разместите файл `webapp/index.html` на любом HTTPS хостинге (GitHub Pages, Vercel, Netlify и т.д.)

В файле `index.html` замените `YOUR_SERVER_URL` на адрес вашего Go сервера (например, `https://your-domain.com`).

### Вариант 2: Docker Compose (рекомендуется для продакшена)

#### 1. Подготовка

Убедитесь, что у вас установлены Docker и Docker Compose.

Создайте файл `.env` в корне проекта:

```bash
cp .env.example .env
```

#### 2. Запуск всех сервисов

```bash
docker-compose up -d
```

Команды поднимут четыре контейнера:
- `union_ai_apple_postgres` (PostgreSQL)
- `ai_apple_classifier` (Python, порт 5000)
- `ai_apple_server` (Go, порт 8080)
- `ai_apple_webapp` (Nginx, порт 80)

#### 3. Проверка статуса

```bash
docker-compose ps
docker-compose logs -f
```

#### 4. Остановка

```bash
docker-compose down
```

## Обучение модели

Для обучения классификатора на собственных данных:

1. Подготовьте датасет со структурой:
```
data/
├── train/
│   ├── healthy_apple/
│   ├── apple_scab/
│   └── ...
└── val/
    ├── healthy_apple/
    ├── apple_scab/
    └── ...
```

2. Запустите обучение:
```bash
cd classifier
python train_classifier.py
```

3. Отредактируйте `train_classifier.py` для указания путей к данным

## API Endpoints

### Go Server

#### POST /classify
Загрузка изображения для анализа

**Request:**
- Content-Type: multipart/form-data
- Body: image (file)

**Response:**
```json
{
  "success": true,
  "prediction": "healthy_apple",
  "confidence": 0.95,
  "top_predictions": [
    {"label": "healthy_apple", "confidence": 0.95},
    {"label": "healthy_leaf", "confidence": 0.03}
  ],
  "recommendation": "🍎 Здоровое яблоко обнаружено!..."
}
```

#### GET /health
Проверка работоспособности сервера

#### POST /chat
Текстовый вопрос (RAG по статьям в `data/`). Требует `X-Telegram-Init-Data`.

**Request:** `Content-Type: application/json`  
**Body:** `{"question": "текст вопроса"}`

**Response:** `{"success": true, "answer": "..."}` — при ошибке `success: false`, поле `error`.

#### POST /session, GET /history, POST /message
Чат с историей (основной UI). Требуют `X-Telegram-Init-Data`.

### Python Classifier Service

#### POST /classify
Классификация изображения

**Request:**
- Content-Type: multipart/form-data
- Body: image (file)

**Response:**
```json
{
  "success": true,
  "prediction": "healthy_apple",
  "confidence": 0.95,
  "top_predictions": [...]
}
```

#### POST /rag/context
Поиск по Chroma; возвращает `context`, `fragments`, `few_shot` для сборки ответа в Go.

**Request:** `{"question": "текст"}`  
**Response:** `{"success": true, "context": "...", "fragments": [...], ...}`

## Требования к изображениям

- Формат: JPEG, PNG
- Максимальный размер: 10 MB
- Рекомендуемое разрешение: от 224x224 пикселей

## Конфигурация LLM

- **И текст RAG, и советы по фото:** `LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL` в Go (см. `.env.example`). Без ключа по фото — шаблоны; текстовый чат вернёт ошибку о необходимости ключа.

## Лицензия

MIT License

## Контакты

Для вопросов и предложений создайте issue в репозитории проекта.
