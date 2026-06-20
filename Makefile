APP_NAME=task-api

.PHONY: run test cover docker-up docker-down migrate fmt

run:
	go run ./cmd/api --config ./config.yaml

test:
	go test ./...

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

docker-up:
	cp -n .env.example .env || true
	docker compose up --build -d

docker-down:
	docker compose down

fmt:
	gofmt -w ./cmd ./internal ./tests
