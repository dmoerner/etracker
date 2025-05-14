DOCKER = docker

# https://stackoverflow.com/a/70663753
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

dev:
	$(DOCKER) compose -f compose.yaml up -d etracker_pg
	npm run build --prefix frontend
	go run ./...

test:
	go test ./...
