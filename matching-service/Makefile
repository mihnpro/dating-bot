.PHONY: build run docker-up docker-down deps clean

build:
	go build -o ./bin/matching-service ./cmd/server

run:
	go run ./cmd/server

deps:
	go mod tidy

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down -v

clean:
	rm -rf ./bin
	go clean
