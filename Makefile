.PHONY: build run test cli test-docker

build:
	go build -o bin/memento-redact ./cmd/server

run: build
	bin/memento-redact

cli:
	go build -o bin/memento-redact-cli ./cmd/cli

test:
	go test ./...

test-docker:
	docker compose -f docker-compose.test.yml run --rm backend-test
	docker compose -f docker-compose.test.yml run --rm frontend-test
