.PHONY: build
build:
	docker compose -f .docker/compose.yml build 

.PHONY: up
up:
	docker compose -f .docker/compose.yml up --remove-orphans 

.PHONY: down
down:
	docker compose -f .docker/compose.yml down
