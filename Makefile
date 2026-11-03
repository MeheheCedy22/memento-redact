.PHONY: build cli web run test docker test-docker

build:
	go build -o bin/memento-redact ./cmd/server

cli:
	go build -o bin/memento-redact-cli ./cmd/cli

web:
	cd web && npm ci && npm run build

run: build web
	bin/memento-redact

test:
	go test ./...
	cd web && npm run build

docker:
	docker build -t memento-redact:local .

test-docker:
	docker compose -f docker-compose.test.yml run --rm backend-test
	docker compose -f docker-compose.test.yml run --rm frontend-test
