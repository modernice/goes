.PHONY: build
build:
	docker compose -f .docker/compose.yml build 

.PHONY: up
up:
	docker compose -f .docker/compose.yml up --abort-on-container-exit --remove-orphans 

.PHONY: down
down:
	docker compose -f .docker/compose.yml down
