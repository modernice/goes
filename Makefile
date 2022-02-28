ifeq "${count}" ""
	count=1
endif

ifeq "${run}" ""
	run=""
endif

ifeq "${bench}" ""
	bench=.
endif

# `make test count=50` to run `go test -race -count=50 ./...`
# `make test run=TestXXX` to run `go test -race -run=TestXXX ./...`
.PHONY: test
test:
	go test -race -run=${run} -count=${count} ./...

.PHONY: nats-test
nats-test:
	docker-compose -f .docker/nats-test.yml up --build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/nats-test.yml down --remove-orphans

.PHONY: nats-bench
nats-bench:
	docker-compose -f .docker/nats-bench.yml up --build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/nats-bench.yml down --remove-orphans

.PHONY: mongo-test
mongo-test:
	docker-compose -f .docker/mongo-test.yml up --build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/mongo-test.yml down --remove-orphans

.PHONY: coverage
coverage:
	docker-compose \
		-f .docker/mongo-test.yml \
		-f .docker/nats-test.yml \
		-f .docker/coverage.yml up \
		--build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/coverage.yml down --remove-orphans; \
	go tool cover -html=out/coverage.out

.PHONY: github-coverage
github-coverage:
	docker-compose \
		-f .docker/mongo-test.yml \
		-f .docker/nats-test.yml \
		-f .docker/coverage.yml up \
		--build --abort-on-container-exit --remove-orphans; \
	docker-compose -f .docker/coverage.yml down; \

.PHONY: bench
bench:
	go test -bench=${bench} -run=${run} -count=${count} ./...

.PHONY: cli
cli:
	go install ./cmd/goes

.PHONY: mock-cli-connector
mock-cli-connector:
	go run ./internal/cmd/cli-connector/main.go

.PHONY: cli-connector
