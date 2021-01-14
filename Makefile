test:
	go test ./...

.PHONY: test

nats-test:
	docker-compose -f .docker/nats-test.yml up --build --abort-on-container-exit --remove-orphans
	docker-compose -f .docker/nats-test.yml down

.PHONY: nats-test
