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

# Tests are not concurrent-safe at this time.
test:
	$(DOCKER) compose -f compose.yaml up -d etracker_pg_test
	go test -p 1 --count=1 ./...
