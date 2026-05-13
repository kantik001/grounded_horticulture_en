.PHONY: build up down restart logs clean ps help

# Имя проекта Docker Compose
PROJECT_NAME := union_ai_apple_app

# Основные команды

## Сборка всех образов
build:
	docker compose -p $(PROJECT_NAME) build --no-cache

## Сборка без кэша (полная пересборка)
build-no-cache:
	docker compose -p $(PROJECT_NAME) build --no-cache --pull

## Запуск всех сервисов в фоновом режиме
up:
	docker compose -p $(PROJECT_NAME) up -d

## Запуск с пересборкой изменённых сервисов
up-build:
	docker compose -p $(PROJECT_NAME) up -d --build

## Запуск в режиме foreground (для отладки)
up-dev:
	docker compose -p $(PROJECT_NAME) up

## Остановка всех сервисов
down:
	docker compose -p $(PROJECT_NAME) down

## Остановка с удалением томов
down-volumes:
	docker compose -p $(PROJECT_NAME) down -v

## Перезапуск всех сервисов
restart:
	docker compose -p $(PROJECT_NAME) restart

## Просмотр логов всех сервисов
logs:
	docker compose -p $(PROJECT_NAME) logs -f

## Просмотр логов конкретного сервиса (пример: make logs-service SERVICE=webapp)
logs-service:
	docker compose -p $(PROJECT_NAME) logs -f $(SERVICE)

## Показать статус сервисов
ps:
	docker compose -p $(PROJECT_NAME) ps

## Очистка: остановка, удаление контейнеров, образов и томов
clean:
	docker compose -p $(PROJECT_NAME) down -v --rmi all --remove-orphans

## Пересборка и запуск одного сервиса (пример: make rebuild SERVICE=webapp)
rebuild:
	docker compose -p $(PROJECT_NAME) up -d --build --force-recreate $(SERVICE)

## Проверка здоровья сервисов
health:
	docker compose -p $(PROJECT_NAME) ps

## Помощь по доступным командам
help:
	@echo "Доступные команды:"
	@echo "  make build          - Сборка всех образов"
	@echo "  make build-no-cache - Полная пересборка без кэша"
	@echo "  make up             - Запуск сервисов в фоне"
	@echo "  make up-build       - Запуск с пересборкой"
	@echo "  make up-dev         - Запуск в режиме отладки (foreground)"
	@echo "  make down           - Остановка сервисов"
	@echo "  make down-volumes   - Остановка с удалением томов"
	@echo "  make restart        - Перезапуск сервисов"
	@echo "  make logs           - Просмотр логов всех сервисов"
	@echo "  make logs-service SERVICE=<name> - Логи конкретного сервиса"
	@echo "  make ps             - Статус сервисов"
	@echo "  make clean          - Полная очистка (контейнеры, образы, тома)"
	@echo "  make rebuild SERVICE=<name> - Пересборка и запуск одного сервиса"
	@echo "  make health         - Проверка статуса сервисов"
	@echo "  make help           - Эта справка"
