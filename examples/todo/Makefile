.PHONY: test
test:
	go test ./...

.PHONY: build
build:
	docker compose -f .docker/compose.yml build 

.PHONY: default
default:
	docker compose -f .docker/compose.yml up --remove-orphans; \
	docker compose -f .docker/compose.yml down --remove-orphans

.PHONY: debounce
debounce:
	TODO_DEBOUNCE=1s docker compose -f .docker/compose.yml up --remove-orphans; \
	docker compose -f .docker/compose.yml down --remove-orphans

.PHONY: down
down:
	docker compose -f .docker/compose.yml down --remove-orphans
