generate:
	go generate ./...

.PHONY: generate

test:
	docker-compose -f .docker/test.yml up --build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/test.yml down

.PHONY: test

build:
	./scripts/build

.PHONY: build

up:
	docker-compose -f .docker/compose.yml up --remove-orphans

.PHONY: up

down:
	docker-compose -f .docker/compose.yml down --remove-orphans

.PHONY: down
