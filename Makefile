.PHONY: build up down restart logs clean ps help test test-go test-py smoke eval-retrieval eval-fast reindex docker-reindex docker-reindex-apply eval-apple eval-pear eval-plum

# Docker Compose project name
PROJECT_NAME := union_ai_apple

# Main commands

## Build all images
build:
	docker compose -p $(PROJECT_NAME) build --no-cache

## Build without cache (full rebuild)
build-no-cache:
	docker compose -p $(PROJECT_NAME) build --no-cache --pull

## Start all services in the background
up:
	docker compose -p $(PROJECT_NAME) up -d

## Start with rebuild of changed services
up-build:
	docker compose -p $(PROJECT_NAME) up -d --build

## Start in foreground (for debugging)
up-dev:
	docker compose -p $(PROJECT_NAME) up

## Stop all services
down:
	docker compose -p $(PROJECT_NAME) down

## Stop and remove volumes
down-volumes:
	docker compose -p $(PROJECT_NAME) down -v

## Restart all services
restart:
	docker compose -p $(PROJECT_NAME) restart

## Tail logs for all services
logs:
	docker compose -p $(PROJECT_NAME) logs -f

## Tail logs for one service (example: make logs-service SERVICE=webapp)
logs-service:
	docker compose -p $(PROJECT_NAME) logs -f $(SERVICE)

## Show service status
ps:
	docker compose -p $(PROJECT_NAME) ps

## Clean: stop, remove containers, images, and volumes
clean:
	docker compose -p $(PROJECT_NAME) down -v --rmi all --remove-orphans

## Rebuild and start one service (example: make rebuild SERVICE=webapp)
rebuild:
	docker compose -p $(PROJECT_NAME) up -d --build --force-recreate $(SERVICE)

## Health check for services
health:
	docker compose -p $(PROJECT_NAME) ps

## Go unit tests
test-go:
	cd server && go test -v -count=1 ./...

## Python unit tests
test-py:
	pip install -r tests/requirements-test.txt
	pytest tests/ -v

test: test-go test-py

## Smoke API (localhost:8080, TELEGRAM_AUTH_DISABLED=true)
smoke:
	powershell -ExecutionPolicy Bypass -File scripts/smoke.ps1

## Reindex Chroma + BM25 (locally)
reindex:
	python scripts/reindex_rag.py

## Rebuild classifier + reindex in Docker volumes chroma_data + bm25_data
docker-reindex:
	docker compose -p $(PROJECT_NAME) build classifier
	docker compose -p $(PROJECT_NAME) run --rm -e FORCE_RAG_REINDEX=true classifier python scripts/reindex_rag.py

## Reindex in Docker + restart classifier (after data/ edits)
docker-reindex-apply:
	$(MAKE) docker-reindex
	docker compose -p $(PROJECT_NAME) restart classifier

## RAG eval retrieval-only (CLASSIFIER_RAG_URL, classifier on :5000)
eval-retrieval:
	pip install requests
	python scripts/run_rag_eval.py --suite all

## Fast smoke-eval in-process without rerank (~20s in Docker)
eval-fast:
	docker compose -p $(PROJECT_NAME) exec classifier python scripts/run_rag_eval.py --suite all --in-process --fast

## RAG eval per crop: make eval-apple | eval-pear | eval-plum
eval-apple:
	python scripts/run_rag_eval.py --suite apple
eval-pear:
	python scripts/run_rag_eval.py --suite pear
eval-plum:
	python scripts/run_rag_eval.py --suite plum

## Help for available commands
help:
	@echo "Available commands:"
	@echo "  make build          - Build all images"
	@echo "  make build-no-cache - Full rebuild without cache"
	@echo "  make up             - Start services in background"
	@echo "  make up-build       - Start with rebuild"
	@echo "  make up-dev         - Start in debug mode (foreground)"
	@echo "  make down           - Stop services"
	@echo "  make down-volumes   - Stop and remove volumes"
	@echo "  make restart        - Restart services"
	@echo "  make logs           - Tail logs for all services"
	@echo "  make logs-service SERVICE=<name> - Logs for one service"
	@echo "  make ps             - Service status"
	@echo "  make clean          - Full cleanup (containers, images, volumes)"
	@echo "  make rebuild SERVICE=<name> - Rebuild and start one service"
	@echo "  make health         - Service health status"
	@echo "  make test-go        - Go unit tests (server/)"
	@echo "  make test-py        - Python unit tests (tests/)"
	@echo "  make test           - test-go + test-py"
	@echo "  make smoke          - Smoke API (localhost:8080)"
	@echo "  make reindex        - Reindex Chroma+BM25 (local; on Windows use docker-reindex)"
	@echo "  make docker-reindex - Reindex in Docker volumes chroma_data + bm25_data"
	@echo "  make docker-reindex-apply - docker-reindex + restart classifier"
	@echo "  make eval-retrieval - RAG eval retrieval-only (all crops)"
	@echo "  make eval-fast      - RAG smoke-eval: in-process + no rerank"
	@echo "  make eval-apple     - RAG eval: apple"
	@echo "  make eval-pear      - RAG eval: pear"
	@echo "  make eval-plum      - RAG eval: plum"
	@echo "  make help           - This help"
