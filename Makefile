VERSION ?= dev
BINARY ?= inkbite
LDFLAGS := -X main.version=$(VERSION)
GOFILES := $(shell git ls-files '*.go')

.PHONY: build test vet fmt ci dist clean

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/inkbite

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(GOFILES)

ci: test vet

dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/inkbite
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/inkbite
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/inkbite

clean:
	rm -rf bin dist coverage.out
