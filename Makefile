.PHONY: bootstrap test vet integration up down
bootstrap:
	go mod tidy
	go mod verify
test:
	go test -race -count=1 ./...
vet:
	go vet ./...
up:
	docker compose up -d --wait
down:
	docker compose down -v
integration:
	COMMON_INTEGRATION=1 go test -race -count=1 ./redis ./nats
	COMMON_POSTGRES_INTEGRATION=1 go test -tags postgres_integration -race -count=1 ./database
