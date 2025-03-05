# BUILD VARIABLES
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := dev
ifdef COMMIT
	COMMIT := $(COMMIT)
else
	COMMIT := $(shell git rev-parse --short=12 HEAD)
endif

default: run

run:
	@go run cmd/cli/main.go --config config.json

build:
	@CGO_ENABLED=0 go build -ldflags="-X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)' -s -w" -o bin/dca-cli ./cmd/cli

build-lambda:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)' -s -w" -tags lambda.norpc -o bin/bootstrap ./cmd/lambda
	@cd bin && zip lambda.zip bootstrap
	@rm bin/bootstrap

test:
	go test ./... -cover

itest:
	go test ./... -cover -tags integration

clean:
	@rm -rf bin/

.PHONY:run build build-lambda test itest clean