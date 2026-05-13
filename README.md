# 🍏 union_ai_apple_project — объединённый помощник садовода

Сборка на базе **ai_apple_support** с добавлением **текстового RAG** из **project_apple_bot**: фото → классификация + советы через Go и LLM; текст → поиск по статьям и ответ в Python (OpenRouter), шлюз запросов — **Go**.

## Описание

- Распознавание яблока/листа (**MobileNetV2** в Python) и рекомендации по фото (**Go** вызывает LLM или шаблоны).
- Текстовые вопросы по статьям из **`data/`** (**Chroma + LangChain + верификация** в Python), вызов с Web App через **`POST /api/chat`**.

## Архитектура

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────────────┐
│  Telegram Web   │────▶│   Go Server      │────▶│ Python (Flask)              │
│     App         │◀────│   /classify      │     │  /classify — CV             │
└─────────────────┘     │   /chat        │────▶│  /chat — RAG + OpenRouter   │
                          └────────┬────────┘     └─────────────────────────────┘
                                   │
                                   ▼ (только рекомендации по фото)
                            ┌──────────────┐
                            │ LLM API      │
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
- **Flask** — HTTP API (`/classify`, `/chat`)
- **LangChain + Chroma + HuggingFace embeddings** — RAG по текстам из `data/`

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

**Текст:** пользователь вводит вопрос и нажимает «Спросить агронома» → `POST /chat` (Go → Python RAG + OpenRouter) → отображается ответ по статьям.

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

См. актуальный перечень в **`.env.example`** (в т.ч. `CLASSIFIER_URL`, `CLASSIFIER_CHAT_URL`, `OPENROUTER_API_KEY` для RAG).

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

Команды поднимут три контейнера:
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
Текстовый вопрос (RAG по статьям в `data/`).

**Request:** `Content-Type: application/json`  
**Body:** `{"question": "текст вопроса"}`

**Response:** `{"success": true, "answer": "..."}` — при ошибке `success: false`, поле `error`.

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

#### POST /chat
То же тело и формат ответа, что у Go; выполняется поиск по Chroma и генерация ответа (нужен **`OPENROUTER_API_KEY`** в окружении).

## Требования к изображениям

- Формат: JPEG, PNG
- Максимальный размер: 10 MB
- Рекомендуемое разрешение: от 224x224 пикселей

## Конфигурация LLM

- **Рекомендации по фото:** переменные `LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL` (по умолчанию OpenRouter). Без ключа — шаблонные тексты в Go.
- **Текстовый RAG:** в Python используется **`OPENROUTER_API_KEY`** (см. `.env.example`). Без него ответы по статьям работать не будут.

## Лицензия

MIT License

## Контакты

Для вопросов и предложений создайте issue в репозитории проекта.
