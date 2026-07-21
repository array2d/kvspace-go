.PHONY: build test vet clean install

export GOPROXY ?= https://goproxy.cn,direct
PREFIX        ?= ~/.local

build:
	go mod tidy
	go build -ldflags="-s -w" -o kvspace ./cmd/kvspace/

install: build
	install -d $(PREFIX)/bin
	install kvspace $(PREFIX)/bin/kvspace

test:
	go test ./... -count=1

vet:
	go vet ./...

clean:
	go clean
	rm -f kvspace
