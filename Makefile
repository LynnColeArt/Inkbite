VERSION ?= dev
BINARY ?= inkbite
LDFLAGS := -X main.version=$(VERSION)
GOFILES := $(shell git ls-files '*.go')

.PHONY: build test vet fmt ci dist clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/inkbite

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(GOFILES)

ci: test vet build

dist:
	./scripts/dist.sh "$(VERSION)" "$(BINARY)"

clean:
	rm -rf bin dist coverage.out
