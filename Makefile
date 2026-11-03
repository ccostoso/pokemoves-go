.PHONY: run build test vet gen gen-check docker lint

run:
	cd api && go run ./cmd/server

build:
	cd api && go build ./...

test:
	cd api && go test ./...

vet:
	cd api && go vet ./...

gen:
	cd api && go tool oapi-codegen -config oapi-codegen.yaml openapi.yaml

gen-check: gen
	git diff --exit-code -- api/internal/openapi

docker:
	docker build -t pokemoves-api ./api

lint:
	cd api && go tool staticcheck ./...