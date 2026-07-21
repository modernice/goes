.PHONY: docs
docs:
	@./scripts/docs.sh

.PHONY: generate
generate:
	@./scripts/generate

.PHONY: test
test:
	@go test $$(bash ./scripts/test-packages ./...)

.PHONY: test-race
test-race:
	@go test -race $$(bash ./scripts/test-packages ./...)

.PHONY: test-actions
test-actions:
	docker build .docker -f .docker/act.Dockerfile -t ghcr.io/catthehacker/ubuntu:full-20.04
	@./scripts/test-actions || true

.PHONY: nats-test
nats-test:
	docker compose -f .docker/nats-test.yml down --remove-orphans -v 2>/dev/null; \
	docker compose -f .docker/nats-test.yml up --build --exit-code-from test --remove-orphans; \
	status=$$?; \
	docker compose -f .docker/nats-test.yml down --remove-orphans -v; \
	exit $$status

.PHONY: nats-bench
nats-bench:
	docker compose -f .docker/nats-bench.yml up --build --abort-on-container-exit --remove-orphans; \
	docker compose -f .docker/nats-bench.yml down --remove-orphans

.PHONY: mongo-test
mongo-test:
	docker compose -f .docker/mongo-test.yml up --build --exit-code-from test --remove-orphans; \
	status=$$?; \
	docker compose -f .docker/mongo-test.yml down --remove-orphans; \
	exit $$status

.PHONY: postgres-test
postgres-test:
	docker compose -f .docker/postgres-test.yml up --build --exit-code-from test --remove-orphans; \
	status=$$?; \
	docker compose -f .docker/postgres-test.yml down --remove-orphans; \
	exit $$status

.PHONY: coverage
coverage:
	docker compose -f .docker/coverage.yml up --build --exit-code-from test --remove-orphans; \
	status=$$?; \
	docker compose -f .docker/coverage.yml down --remove-orphans; \
	[ $$status -eq 0 ] || exit $$status; \
	go tool cover -html=out/coverage.out

.PHONY: github-test
github-test:
	docker compose -f .docker/github.yml up --build --abort-on-container-exit --remove-orphans && \
	docker compose -f .docker/github.yml down --remove-orphans; \

.PHONY: bench
bench:
	@go test -bench=${bench} -run=${run} -count=${count} $$(bash ./scripts/test-packages ./...)

.PHONY: cli
cli:
	@go install ./cmd/goes

.PHONY: mock-cli-connector
mock-cli-connector:
	@go run ./internal/cmd/cli-connector/main.go
