ifeq "${count}" ""
	count=1
endif

# `make test count=50` to run `go test -v -race -count=50 ./...`
test:
	go test -v -race -count=${count} ./...

.PHONY: test

nats-test:
	docker-compose -f .docker/nats-test.yml up --build --abort-on-container-exit --remove-orphans
	docker-compose -f .docker/nats-test.yml down

.PHONY: nats-test

coverage:
	docker-compose -f .docker/coverage.yml up --build --abort-on-container-exit --remove-orphans
	docker-compose -f .docker/coverage.yml down
	go tool cover -html=out/coverage.out

.PHONY: coverage
